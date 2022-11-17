package controller

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1coreinformerfactory "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	nadinformers "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/informers/externalversions"
	nadlisterv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/listers/k8s.cni.cncf.io/v1"
	nadutils "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/utils"
	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/annotations"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/logging"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/multuscni"
)

const (
	AdvertisedName                              = "pod-networks-updates"
	maxRetries                                  = 2
	add            DynamicAttachmentRequestType = "add"
	remove         DynamicAttachmentRequestType = "remove"
)

type DynamicAttachmentRequestType string

type DynamicAttachmentRequest struct {
	Pod         *corev1.Pod
	Attachments []*nadv1.NetworkSelectionElement
	Type        DynamicAttachmentRequestType
	PodNetNS    string
}

func (dar *DynamicAttachmentRequest) String() string {
	req, err := json.Marshal(dar)
	if err != nil {
		klog.Warningf("failed to marshal DynamicAttachmentRequest: %v", err)
		return ""
	}
	return string(req)
}

// PodNetworksController handles the cncf networks annotations update, and
// triggers adding / removing networks from a running pod.
type PodNetworksController struct {
	k8sClientSet            kubernetes.Interface
	arePodsSynched          cache.InformerSynced
	areNetAttachDefsSynched cache.InformerSynced
	podsInformer            cache.SharedIndexInformer
	netAttachDefInformer    cache.SharedIndexInformer
	podsLister              v1corelisters.PodLister
	netAttachDefLister      nadlisterv1.NetworkAttachmentDefinitionLister
	broadcaster             record.EventBroadcaster
	recorder                record.EventRecorder
	workqueue               workqueue.RateLimitingInterface
	nadClientSet            nadclient.Interface
	containerRuntime        cri.ContainerRuntime
	multusClient            multuscni.Client
}

// NewPodNetworksController returns new PodNetworksController instance
func NewPodNetworksController(
	k8sCoreInformerFactory v1coreinformerfactory.SharedInformerFactory,
	nadInformers nadinformers.SharedInformerFactory,
	broadcaster record.EventBroadcaster,
	recorder record.EventRecorder,
	k8sClientSet kubernetes.Interface,
	nadClientSet nadclient.Interface,
	containerRuntime cri.ContainerRuntime,
	multusClient multuscni.Client,
) (*PodNetworksController, error) {
	podInformer := k8sCoreInformerFactory.Core().V1().Pods().Informer()
	nadInformer := nadInformers.K8sCniCncfIo().V1().NetworkAttachmentDefinitions().Informer()

	podNetworksController := &PodNetworksController{
		arePodsSynched:          podInformer.HasSynced,
		areNetAttachDefsSynched: nadInformer.HasSynced,
		podsInformer:            podInformer,
		podsLister:              k8sCoreInformerFactory.Core().V1().Pods().Lister(),
		netAttachDefInformer:    nadInformer,
		netAttachDefLister:      nadInformers.K8sCniCncfIo().V1().NetworkAttachmentDefinitions().Lister(),
		recorder:                recorder,
		broadcaster:             broadcaster,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			AdvertisedName),
		k8sClientSet:     k8sClientSet,
		nadClientSet:     nadClientSet,
		containerRuntime: containerRuntime,
		multusClient:     multusClient,
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: podNetworksController.handlePodUpdate,
	})

	return podNetworksController, nil
}

// Start runs worker thread after performing cache synchronization
func (pnc *PodNetworksController) Start(stopChan <-chan struct{}) {
	klog.Infof("starting network controller")
	defer pnc.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopChan, pnc.arePodsSynched, pnc.areNetAttachDefsSynched); !ok {
		klog.Infof("failed waiting for caches to sync")
	}

	go wait.Until(pnc.worker, time.Second, stopChan)
	<-stopChan
	klog.Infof("shutting down network controller")
}

func (pnc *PodNetworksController) worker() {
	for pnc.processNextWorkItem() {
	}
}

func (pnc *PodNetworksController) processNextWorkItem() bool {
	queueItem, shouldQuit := pnc.workqueue.Get()
	if shouldQuit {
		return false
	}
	defer pnc.workqueue.Done(queueItem)

	dynAttachmentRequest := queueItem.(*DynamicAttachmentRequest)
	klog.Infof("extracted request [%v] from the queue", dynAttachmentRequest)
	err := pnc.handleDynamicInterfaceRequest(dynAttachmentRequest)
	pnc.handleResult(err, dynAttachmentRequest)

	return true
}

func (pnc *PodNetworksController) handleDynamicInterfaceRequest(dynamicAttachmentRequest *DynamicAttachmentRequest) error {
	klog.Infof("handleDynamicInterfaceRequest: read from queue: %v", dynamicAttachmentRequest)
	if dynamicAttachmentRequest.Type == add {
		return pnc.addNetworks(dynamicAttachmentRequest)
	} else if dynamicAttachmentRequest.Type == remove {
		return pnc.removeNetworks(dynamicAttachmentRequest)
	} else {
		klog.Infof("very weird attachment request: %+v", dynamicAttachmentRequest)
	}
	klog.Infof("handleDynamicInterfaceRequest: exited & successfully processed: %v", dynamicAttachmentRequest)
	return nil
}

func (pnc *PodNetworksController) handleResult(err error, dynamicAttachmentRequest *DynamicAttachmentRequest) {
	if err == nil {
		pnc.workqueue.Forget(dynamicAttachmentRequest)
		return
	}

	currentRetries := pnc.workqueue.NumRequeues(dynamicAttachmentRequest)
	if currentRetries <= maxRetries {
		klog.Errorf("re-queued request for: %v. Error: %v", dynamicAttachmentRequest, err)
		pnc.workqueue.AddRateLimited(dynamicAttachmentRequest)
		return
	}

	pnc.workqueue.Forget(dynamicAttachmentRequest)
}

func (pnc *PodNetworksController) handlePodUpdate(oldObj interface{}, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	const (
		add    DynamicAttachmentRequestType = "add"
		remove DynamicAttachmentRequestType = "remove"
	)

	if !didNetworkSelectionElementsChange(oldPod, newPod) {
		return
	}
	podNamespace := oldPod.GetNamespace()
	podName := oldPod.GetName()
	klog.V(logging.Debug).Infof("pod [%s] updated", annotations.NamespacedName(podNamespace, podName))

	oldNetworkSelectionElements, err := networkSelectionElements(oldPod.Annotations, podNamespace)
	if err != nil {
		klog.Errorf("failed to compute the network selection elements from the *old* pod")
		return
	}

	newNetworkSelectionElements, err := networkSelectionElements(newPod.Annotations, podNamespace)
	if err != nil {
		klog.Errorf("failed to compute the network selection elements from the *new* pod")
		return
	}

	toAdd := exclusiveNetworks(newNetworkSelectionElements, oldNetworkSelectionElements)
	klog.Infof("%d attachments to add to pod %s", len(toAdd), annotations.NamespacedName(podNamespace, podName))

	netnsPath, err := pnc.netnsPath(newPod)
	if err != nil {
		klog.Errorf("failed to figure out the pod's network namespace: %v", err)
		return
	}
	if len(toAdd) > 0 {
		pnc.workqueue.Add(
			&DynamicAttachmentRequest{
				Pod:         newPod,
				Attachments: toAdd,
				Type:        add,
				PodNetNS:    netnsPath,
			})
	}

	toRemove := exclusiveNetworks(oldNetworkSelectionElements, newNetworkSelectionElements)
	klog.Infof("%d attachments to remove from pod %s", len(toRemove), annotations.NamespacedName(podNamespace, podName))
	if len(toRemove) > 0 {
		pnc.workqueue.Add(
			&DynamicAttachmentRequest{
				Pod:         newPod,
				Attachments: toRemove,
				Type:        remove,
				PodNetNS:    netnsPath,
			})
	}
}

func (pnc *PodNetworksController) addNetworks(dynamicAttachmentRequest *DynamicAttachmentRequest) error {
	pod := dynamicAttachmentRequest.Pod

	var attachmentResults []annotations.AttachmentResult
	for i := range dynamicAttachmentRequest.Attachments {
		netToAdd := dynamicAttachmentRequest.Attachments[i]
		klog.Infof("network to add: %v", netToAdd)

		netAttachDef, err := pnc.netAttachDefLister.NetworkAttachmentDefinitions(netToAdd.Namespace).Get(netToAdd.Name)
		if err != nil {
			klog.Errorf("failed to access the networkattachmentdefinition %s/%s: %v", netToAdd.Namespace, netToAdd.Name, err)
			return err
		}
		netAttachDefWithDefaults, err := serializeNetAttachDefWithDefaults(netAttachDef)
		if err != nil {
			return err
		}
		response, err := pnc.multusClient.InvokeDelegate(
			multusapi.CreateDelegateRequest(
				multuscni.CmdAdd,
				podContainerID(pod),
				dynamicAttachmentRequest.PodNetNS,
				netToAdd.InterfaceRequest,
				pod.GetNamespace(),
				pod.GetName(),
				string(pod.UID),
				netAttachDefWithDefaults,
				interfaceAttributes(*netToAdd),
			))

		if err != nil {
			return fmt.Errorf("failed to ADD delegate: %v", err)
		}
		klog.Infof("response: %v", *response.Result)

		attachmentResults = append(attachmentResults, *annotations.NewAttachmentResult(netToAdd, response))
		pnc.Eventf(pod, corev1.EventTypeNormal, "AddedInterface", addIfaceEventFormat(pod, netToAdd))
		klog.Infof(
			"added interface %s to pod %s",
			annotations.NamespacedName(netToAdd.Namespace, netToAdd.Name),
			annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		)
	}

	newIfaceStatus, err := annotations.AddDynamicIfaceToStatus(pod, attachmentResults...)
	if err != nil {
		return fmt.Errorf("failed to compute the updated network status: %v", err)
	}

	if err := nadutils.SetNetworkStatus(pnc.k8sClientSet, pod, newIfaceStatus); err != nil {
		return err
	}

	return nil
}

func (pnc *PodNetworksController) removeNetworks(dynamicAttachmentRequest *DynamicAttachmentRequest) error {
	pod := dynamicAttachmentRequest.Pod

	var removedNets []*nadv1.NetworkSelectionElement
	for i := range dynamicAttachmentRequest.Attachments {
		netToRemove := dynamicAttachmentRequest.Attachments[i]
		klog.Infof("network to remove: %v", dynamicAttachmentRequest.Attachments[i])

		netAttachDef, err := pnc.netAttachDefLister.NetworkAttachmentDefinitions(netToRemove.Namespace).Get(netToRemove.Name)
		if err != nil {
			klog.Errorf("failed to access the network-attachment-definition %s/%s: %v", netToRemove.Namespace, netToRemove.Name, err)
			return err
		}

		netAttachDefWithDefaults, err := serializeNetAttachDefWithDefaults(netAttachDef)
		if err != nil {
			return err
		}
		_, err = pnc.multusClient.InvokeDelegate(
			multusapi.CreateDelegateRequest(
				multuscni.CmdDel,
				podContainerID(pod),
				dynamicAttachmentRequest.PodNetNS,
				netToRemove.InterfaceRequest,
				pod.GetNamespace(),
				pod.GetName(),
				string(pod.UID),
				netAttachDefWithDefaults,
				interfaceAttributes(*netToRemove),
			))
		if err != nil {
			return fmt.Errorf("failed to remove delegate: %v", err)
		}

		removedNets = append(removedNets, netToRemove)
		pnc.Eventf(pod, corev1.EventTypeNormal, "RemovedInterface", removeIfaceEventFormat(pod, netToRemove))
		klog.Infof(
			"removed interface %s from pod %s",
			annotations.NamespacedName(netToRemove.Namespace, netToRemove.Name),
			annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		)
	}

	newIfaceStatus, err := annotations.DeleteDynamicIfaceFromStatus(pod, removedNets...)
	if err != nil {
		return fmt.Errorf(
			"failed to compute the dynamic network attachments after unplugging interface for pod %s: %v",
			annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
			err,
		)
	}
	if err := nadutils.SetNetworkStatus(pnc.k8sClientSet, pod, newIfaceStatus); err != nil {
		return err
	}

	return nil
}

func networkSelectionElements(podAnnotations map[string]string, podNamespace string) ([]*nadv1.NetworkSelectionElement, error) {
	podNetworks, ok := podAnnotations[nadv1.NetworkAttachmentAnnot]
	if !ok || podNetworks == "" {
		return []*nadv1.NetworkSelectionElement{}, nil
	}
	podNetworkSelectionElements, err := annotations.ParsePodNetworkAnnotations(podNetworks, podNamespace)
	if err != nil {
		klog.Errorf("failed to extract the network selection elements: %v", err)
		return nil, err
	}
	return podNetworkSelectionElements, nil
}

func networkStatus(podAnnotations map[string]string) ([]nadv1.NetworkStatus, error) {
	podNetworkstatus, ok := podAnnotations[nadv1.NetworkStatusAnnot]
	if !ok {
		return nil, fmt.Errorf("the pod is missing the \"%s\" annotation on its annotations: %+v", nadv1.NetworkStatusAnnot, podAnnotations)
	}
	var netStatus []nadv1.NetworkStatus
	if err := json.Unmarshal([]byte(podNetworkstatus), &netStatus); err != nil {
		return nil, err
	}

	return netStatus, nil
}

func exclusiveNetworks(
	needles []*nadv1.NetworkSelectionElement,
	haystack []*nadv1.NetworkSelectionElement) []*nadv1.NetworkSelectionElement {
	setOfNeedles := indexNetworkSelectionElements(needles)
	haystackSet := indexNetworkSelectionElements(haystack)

	var unmatchedNetworks []*nadv1.NetworkSelectionElement
	for needleNetName, needle := range setOfNeedles {
		if _, ok := haystackSet[needleNetName]; !ok {
			unmatchedNetworks = append(unmatchedNetworks, needle)
		}
	}
	return unmatchedNetworks
}

func indexNetworkSelectionElements(list []*nadv1.NetworkSelectionElement) map[string]*nadv1.NetworkSelectionElement {
	indexedNetworkSelectionElements := make(map[string]*nadv1.NetworkSelectionElement)
	for k := range list {
		indexedNetworkSelectionElements[networkSelectionElementIndexKey(*list[k])] = list[k]
	}
	return indexedNetworkSelectionElements
}

func networkSelectionElementIndexKey(netSelectionElement nadv1.NetworkSelectionElement) string {
	if netSelectionElement.InterfaceRequest != "" {
		return fmt.Sprintf(
			"%s/%s/%s",
			netSelectionElement.Namespace,
			netSelectionElement.Name,
			netSelectionElement.InterfaceRequest)
	}

	return fmt.Sprintf(
		"%s/%s",
		netSelectionElement.Namespace,
		netSelectionElement.Name)
}

// Eventf puts event into kubernetes events
func (pnc *PodNetworksController) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if pnc != nil && pnc.recorder != nil {
		pnc.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
	}
}

func (pnc *PodNetworksController) netnsPath(pod *corev1.Pod) (string, error) {
	if containerID := podContainerID(pod); containerID != "" {
		netns, err := pnc.containerRuntime.NetNS(containerID)
		if err != nil {
			return "", fmt.Errorf("failed to get netns for container [%s]: %w", containerID, err)
		}
		return netns, nil
	}
	return "", nil
}

func podContainerID(pod *corev1.Pod) string {
	cidURI := pod.Status.ContainerStatuses[0].ContainerID
	// format is docker://<cid>
	parts := strings.Split(cidURI, "//")
	if len(parts) > 1 {
		return parts[1]
	}
	return cidURI
}

func addIfaceEventFormat(pod *corev1.Pod, network *nadv1.NetworkSelectionElement) string {
	return fmt.Sprintf(
		"pod [%s]: added interface %s to network: %s",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		network.InterfaceRequest,
		network.Name,
	)
}

func removeIfaceEventFormat(pod *corev1.Pod, network *nadv1.NetworkSelectionElement) string {
	return fmt.Sprintf(
		"pod [%s]: removed interface %s from network: %s",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		network.InterfaceRequest,
		network.Name,
	)
}

func interfaceAttributes(networkData nadv1.NetworkSelectionElement) *multusapi.DelegateInterfaceAttributes {
	if len(networkData.IPRequest) > 0 || networkData.MacRequest != "" {
		return &multusapi.DelegateInterfaceAttributes{
			IPRequest:  networkData.IPRequest,
			MacRequest: networkData.MacRequest,
		}
	}
	return nil
}

func serializeNetAttachDefWithDefaults(netAttachDef *nadv1.NetworkAttachmentDefinition) ([]byte, error) {
	netAttachDefWithDefaults, err := nadutils.GetCNIConfigFromSpec(netAttachDef.Spec.Config, netAttachDef.GetName())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to apply defaults to the net-attach-def %s: %v",
			annotations.NamespacedName(netAttachDef.GetNamespace(), netAttachDef.GetName()),
			err,
		)
	}
	return netAttachDefWithDefaults, nil
}

func didNetworkSelectionElementsChange(oldPod *corev1.Pod, newPod *corev1.Pod) bool {
	oldNetworkSelectionElementsString, didPodHaveExtraAttachments := oldPod.Annotations[nadv1.NetworkAttachmentAnnot]
	newNetworkSelectionElementsString, doesPodHaveExtraAttachmentsNow := newPod.Annotations[nadv1.NetworkAttachmentAnnot]

	if didPodHaveExtraAttachments != doesPodHaveExtraAttachmentsNow ||
		oldNetworkSelectionElementsString != newNetworkSelectionElementsString {
		return true
	}
	return false
}
