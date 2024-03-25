package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/server/api"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/annotations"
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
	Pod          *corev1.Pod
	Attachments  []nadv1.NetworkSelectionElement
	Type         DynamicAttachmentRequestType
	PodSandboxID string
	PodNetNS     string
}

// ContainerRuntime interface
type ContainerRuntime interface {
	// NetworkNamespace returns the network namespace of the given pod.
	NetworkNamespace(ctx context.Context, podUID string) (string, error)
	// PodSandboxID returns the PodSandboxID of the given pod.
	PodSandboxID(ctx context.Context, podUID string) (string, error)
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
	containerRuntime        ContainerRuntime
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
	containerRuntime ContainerRuntime,
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

	if _, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: podNetworksController.handlePodUpdate,
	}); err != nil {
		return nil, fmt.Errorf("error setting the add event handlers: %v", err)
	}

	return podNetworksController, nil
}

// Start runs worker thread after performing cache synchronization
func (pnc *PodNetworksController) Start(stopChan <-chan struct{}) {
	klog.Infof("starting network controller")
	defer pnc.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopChan, pnc.arePodsSynched, pnc.areNetAttachDefsSynched); !ok {
		klog.Infof("failed waiting for caches to sync")
		return
	}

	// ensure that we didn't miss any updates before the cache sync completion
	if err := pnc.reconcileOnStartup(); err != nil {
		klog.Infof("failed to reconcile pods on startup: %v", err)
		return
	}

	go wait.Until(pnc.worker, time.Second, stopChan)
	<-stopChan
	klog.Infof("shutting down network controller")
}

func (pnc *PodNetworksController) worker() {
	for pnc.processNextWorkItem() {
	}
}

func (pnc *PodNetworksController) ignoreHostNetworkedPods(pod *corev1.Pod) bool {
	// since there is no such "not has" relation in a field selector,
	// filter out pods that are of no concern to the controller here
	if pod.Spec.HostNetwork {
		_, haveNetworkAttachments := pod.GetAnnotations()[nadv1.NetworkAttachmentAnnot]
		namespacedName := annotations.NamespacedName(pod.GetNamespace(), pod.GetName())
		if haveNetworkAttachments {
			klog.Warningf("rejecting to add interfaces for host networked pod: %s", namespacedName)
			pnc.Eventf(pod, corev1.EventTypeWarning, "InterfaceAddRejected", rejectInterfaceAddEventFormat(pod))
		} else {
			klog.V(logging.Debug).Infof("host networked pod [%s] got filtered out", namespacedName)
		}
		return true
	}
	return false
}

func (pnc *PodNetworksController) reconcileOnStartup() error {
	pods, err := pnc.podsLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pods on current node: %v", err)
	}
	for _, pod := range pods {
		if pnc.ignoreHostNetworkedPods(pod) {
			continue
		}
		namespacedName := annotations.NamespacedName(pod.GetNamespace(), pod.GetName())
		klog.V(logging.Debug).Infof("pod [%s] added to reconcile on startup", namespacedName)
		pnc.workqueue.Add(&namespacedName)
	}
	return nil
}

func (pnc *PodNetworksController) processNextWorkItem() bool {
	queueItem, shouldQuit := pnc.workqueue.Get()
	if shouldQuit {
		return false
	}
	ctx := context.Background()

	defer pnc.workqueue.Done(queueItem)
	podNamespacedName := queueItem.(*string)
	klog.Infof("extracted update request for pod [%s] from the queue", *podNamespacedName)
	podNamespace, podName, err := separateNamespaceAndName(*podNamespacedName)
	if err != nil {
		klog.Errorf("the update key - [%s] - is not in the namespaced name format: %v", *podNamespacedName, err)
		return true
	}

	var results []annotations.AttachmentResult
	var pod *corev1.Pod
	defer func() {
		pnc.handleResult(err, podNamespacedName, pod, results)
	}()

	pod, err = pnc.podsLister.Pods(podNamespace).Get(podName)
	if err != nil {
		klog.Errorf("could not access pod from the informer")
		return true
	}

	networkSelectionElements, networkStatus, err := getPodNetworks(pod)
	if err != nil {
		klog.Errorf("failed to get pod networks: %v", err)
		return true
	}
	indexedNetworkSelectionElements := annotations.IndexNetworkSelectionElements(networkSelectionElements)
	indexedNetworkStatus := annotations.IndexNetworkStatus(networkStatus)

	netnsPath, err := pnc.containerRuntime.NetworkNamespace(ctx, string(pod.UID))
	if err != nil {
		klog.Errorf("failed to figure out the pod's network namespace: %v", err)
		return true
	}

	podSandboxID, err := pnc.containerRuntime.PodSandboxID(ctx, string(pod.UID))
	if err != nil {
		klog.Errorf("failed to figure out the PodSandboxID: %v", err)
		return true
	}

	// The order in which the attachments will be added must be maintained.
	// Having a deterministic order helps for troubleshooting and testing.
	// It is also probably required by CNI due, example:
	// A macvlan (net-1) is added as an attachment, and a VLAN (net-2) is added as a separated
	// attachment and has linkInContainer set to true and master set to net-1. If the VLAN is added
	// before the macvlan, then it will fail.
	var attachmentsToAdd []nadv1.NetworkSelectionElement
	for i := range networkSelectionElements {
		if _, wasFound := indexedNetworkStatus[annotations.NetworkSelectionElementIndexKey(networkSelectionElements[i])]; !wasFound {
			attachmentsToAdd = append(attachmentsToAdd, networkSelectionElements[i])
		}
	}

	if len(attachmentsToAdd) > 0 {
		results, err = pnc.handleDynamicInterfaceRequest(
			&DynamicAttachmentRequest{
				Pod:          pod,
				Attachments:  attachmentsToAdd,
				Type:         add,
				PodNetNS:     netnsPath,
				PodSandboxID: podSandboxID,
			})
		if err != nil {
			klog.Errorf("error adding attachments: %v", err)
			return true
		}
	}

	// The order in which the attachments will be removed doesn't have to be maintained since CNI DEL must be very permissive
	// in case of an error (e.g.: Interface already deleted by another attachement should not produce any error).
	// For troubleshooting and testing, having a deterministic behavior is preferred.
	var attachmentsToRemove []nadv1.NetworkSelectionElement
	for i := range networkStatus {
		networkNamespace, networkName, _ := separateNamespaceAndName(annotations.NetworkStatusIndexKey(networkStatus[i]))
		if _, wasFound := indexedNetworkSelectionElements[annotations.NetworkStatusIndexKey(networkStatus[i])]; !wasFound {
			attachmentsToRemove = append(attachmentsToRemove, nadv1.NetworkSelectionElement{
				Name:             networkName,
				Namespace:        networkNamespace,
				InterfaceRequest: networkStatus[i].Interface,
			})
		}
	}
	if len(attachmentsToRemove) > 0 {
		var res []annotations.AttachmentResult
		res, err = pnc.handleDynamicInterfaceRequest(&DynamicAttachmentRequest{
			Pod:          pod,
			Attachments:  attachmentsToRemove,
			Type:         remove,
			PodNetNS:     netnsPath,
			PodSandboxID: podSandboxID,
		})
		results = append(results, res...)
		if err != nil {
			klog.Errorf("error removing attachments: %v", err)
			return true
		}
	}

	return true
}

func (pnc *PodNetworksController) handleDynamicInterfaceRequest(
	dynamicAttachmentRequest *DynamicAttachmentRequest,
) ([]annotations.AttachmentResult, error) {
	klog.Infof("handleDynamicInterfaceRequest: read from queue: %v", dynamicAttachmentRequest)
	if dynamicAttachmentRequest.Type == add {
		return pnc.addNetworks(dynamicAttachmentRequest)
	} else if dynamicAttachmentRequest.Type == remove {
		return pnc.removeNetworks(dynamicAttachmentRequest)
	} else {
		klog.Infof("very weird attachment request: %+v", dynamicAttachmentRequest)
	}
	klog.Infof("handleDynamicInterfaceRequest: exited & successfully processed: %v", dynamicAttachmentRequest)
	return nil, nil
}

func (pnc *PodNetworksController) handleResult(
	err error,
	namespacedPodName *string,
	pod *corev1.Pod,
	results []annotations.AttachmentResult,
) {
	namespacedPodNameString := "<nil>"
	if namespacedPodName != nil {
		namespacedPodNameString = *namespacedPodName
	}

	if results != nil {
		updatedStatus, podNetworkStatusUpdateError := annotations.UpdatePodNetworkStatus(pod, results)
		if podNetworkStatusUpdateError != nil {
			klog.Errorf(
				"error computing pod %s updated network status: %v",
				namespacedPodNameString,
				podNetworkStatusUpdateError,
			)
		}

		if setNetworkStatusError := nadutils.SetNetworkStatus(
			pnc.k8sClientSet,
			pod,
			updatedStatus,
		); setNetworkStatusError != nil {
			klog.Errorf("error updating pod %s network status: %v", namespacedPodNameString, setNetworkStatusError)
		}
	}

	if err == nil {
		pnc.workqueue.Forget(namespacedPodName)
		return
	}

	currentRetries := pnc.workqueue.NumRequeues(namespacedPodName)
	if currentRetries <= maxRetries {
		klog.Errorf("re-queued request for: %s. Error: %v", *namespacedPodName, err)
		pnc.workqueue.AddRateLimited(namespacedPodName)
		return
	}

	pnc.workqueue.Forget(namespacedPodName)
}

func (pnc *PodNetworksController) handlePodUpdate(oldObj interface{}, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	if pnc.ignoreHostNetworkedPods(newPod) {
		return
	}
	if !didNetworkSelectionElementsChange(oldPod, newPod) {
		return
	}

	namespacedName := annotations.NamespacedName(oldPod.GetNamespace(), oldPod.GetName())
	klog.V(logging.Debug).Infof("pod [%s] updated", namespacedName)

	pnc.workqueue.Add(&namespacedName)
}

func (pnc *PodNetworksController) addNetworks(dynamicAttachmentRequest *DynamicAttachmentRequest) ([]annotations.AttachmentResult, error) {
	pod := dynamicAttachmentRequest.Pod

	var attachmentResults []annotations.AttachmentResult
	for i := range dynamicAttachmentRequest.Attachments {
		netToAdd := dynamicAttachmentRequest.Attachments[i]
		klog.Infof("network to add: %v", netToAdd)
		failedAddingEvent := func() {
			pnc.Eventf(pod, corev1.EventTypeWarning, "FailedAddingInterface", failedAddingIfaceEventFormat(pod, &netToAdd))
		}

		netAttachDef, err := pnc.netAttachDefLister.NetworkAttachmentDefinitions(netToAdd.Namespace).Get(netToAdd.Name)
		if err != nil {
			failedAddingEvent()
			klog.Errorf("failed to access the networkattachmentdefinition %s/%s: %v", netToAdd.Namespace, netToAdd.Name, err)
			return attachmentResults, err
		}
		netAttachDefWithDefaults, err := serializeNetAttachDefWithDefaults(netAttachDef)
		if err != nil {
			failedAddingEvent()
			return attachmentResults, err
		}
		response, err := pnc.multusClient.InvokeDelegate(
			multusapi.CreateDelegateRequest(
				multuscni.CmdAdd,
				dynamicAttachmentRequest.PodSandboxID,
				dynamicAttachmentRequest.PodNetNS,
				netToAdd.InterfaceRequest,
				pod.GetNamespace(),
				pod.GetName(),
				string(pod.UID),
				netAttachDefWithDefaults,
				interfaceAttributes(netToAdd),
			))

		if err != nil {
			failedAddingEvent()
			return attachmentResults, fmt.Errorf("failed to ADD delegate: %v", err)
		}
		klog.Infof("response: %v", *response.Result)

		attachmentResults = append(attachmentResults, *annotations.NewAttachmentResult(&netToAdd, response))
		pnc.Eventf(pod, corev1.EventTypeNormal, "AddedInterface", addIfaceEventFormat(pod, &netToAdd))
		klog.Infof(
			"added interface %s to pod %s",
			annotations.NetworkSelectionElementIndexKey(netToAdd),
			annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		)
	}

	return attachmentResults, nil
}

func (pnc *PodNetworksController) removeNetworks(
	dynamicAttachmentRequest *DynamicAttachmentRequest,
) ([]annotations.AttachmentResult, error) {
	pod := dynamicAttachmentRequest.Pod

	var attachmentResults []annotations.AttachmentResult
	for i := range dynamicAttachmentRequest.Attachments {
		netToRemove := dynamicAttachmentRequest.Attachments[i]
		klog.Infof("network to remove: %v", dynamicAttachmentRequest.Attachments[i])

		failedRemovingEvent := func() {
			pnc.Eventf(pod, corev1.EventTypeWarning, "FailedRemovingInterface", failedRemovingIfaceEventFormat(pod, &netToRemove))
		}

		netAttachDef, err := pnc.netAttachDefLister.NetworkAttachmentDefinitions(netToRemove.Namespace).Get(netToRemove.Name)
		if err != nil {
			failedRemovingEvent()
			klog.Errorf("failed to access the network-attachment-definition %s/%s: %v", netToRemove.Namespace, netToRemove.Name, err)
			return attachmentResults, err
		}

		netAttachDefWithDefaults, err := serializeNetAttachDefWithDefaults(netAttachDef)
		if err != nil {
			failedRemovingEvent()
			return attachmentResults, err
		}
		_, err = pnc.multusClient.InvokeDelegate(
			multusapi.CreateDelegateRequest(
				multuscni.CmdDel,
				dynamicAttachmentRequest.PodSandboxID,
				dynamicAttachmentRequest.PodNetNS,
				netToRemove.InterfaceRequest,
				pod.GetNamespace(),
				pod.GetName(),
				string(pod.UID),
				netAttachDefWithDefaults,
				interfaceAttributes(netToRemove),
			))
		if err != nil {
			failedRemovingEvent()
			return attachmentResults, fmt.Errorf("failed to remove delegate: %v", err)
		}

		attachmentResults = append(attachmentResults, *annotations.NewAttachmentResult(&netToRemove, nil))
		pnc.Eventf(pod, corev1.EventTypeNormal, "RemovedInterface", removeIfaceEventFormat(pod, &netToRemove))
		klog.Infof(
			"removed interface %s from pod %s",
			annotations.NetworkSelectionElementIndexKey(netToRemove),
			annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		)
	}

	return attachmentResults, nil
}

// Eventf puts event into kubernetes events
func (pnc *PodNetworksController) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if pnc != nil && pnc.recorder != nil {
		pnc.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
	}
}

func addIfaceEventFormat(pod *corev1.Pod, network *nadv1.NetworkSelectionElement) string {
	attributes := ""
	if len(network.IPRequest) > 0 || network.MacRequest != "" || network.CNIArgs != nil {
		attributes = fmt.Sprintf("(ips: %v, mac: %s, cni-args: %v)",
			network.IPRequest,
			network.MacRequest,
			network.CNIArgs,
		)
	}
	return fmt.Sprintf(
		"pod [%s]: added interface %s to network: %s%s",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		network.InterfaceRequest,
		network.Name,
		attributes,
	)
}

func failedAddingIfaceEventFormat(pod *corev1.Pod, network *nadv1.NetworkSelectionElement) string {
	attributes := ""
	if len(network.IPRequest) > 0 || network.MacRequest != "" || network.CNIArgs != nil {
		attributes = fmt.Sprintf("(ips: %v, mac: %s, cni-args: %v)",
			network.IPRequest,
			network.MacRequest,
			network.CNIArgs,
		)
	}
	return fmt.Sprintf(
		"pod [%s]: failed adding interface %s to network: %s%s",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		network.InterfaceRequest,
		network.Name,
		attributes,
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

func failedRemovingIfaceEventFormat(pod *corev1.Pod, network *nadv1.NetworkSelectionElement) string {
	return fmt.Sprintf(
		"pod [%s]: failed removing interface %s from network: %s",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
		network.InterfaceRequest,
		network.Name,
	)
}

func rejectInterfaceAddEventFormat(pod *corev1.Pod) string {
	return fmt.Sprintf(
		"pod [%s]: will not add interface to host networked pod",
		annotations.NamespacedName(pod.GetNamespace(), pod.GetName()),
	)
}

func interfaceAttributes(networkData nadv1.NetworkSelectionElement) *multusapi.DelegateInterfaceAttributes {
	if len(networkData.IPRequest) > 0 || networkData.MacRequest != "" || networkData.CNIArgs != nil {
		return &multusapi.DelegateInterfaceAttributes{
			IPRequest:  networkData.IPRequest,
			MacRequest: networkData.MacRequest,
			CNIArgs:    networkData.CNIArgs,
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

func separateNamespaceAndName(namespacedName string) (namespace string, name string, err error) {
	splitNamespacedName := strings.Split(namespacedName, "/")
	if len(splitNamespacedName) != 2 && len(splitNamespacedName) != 3 {
		return "", "", fmt.Errorf("invalid namespaced name: %s", namespacedName)
	}
	return splitNamespacedName[0], splitNamespacedName[1], nil
}

func getPodNetworks(pod *corev1.Pod) ([]nadv1.NetworkSelectionElement, []nadv1.NetworkStatus, error) {
	networkSelectionElements, err := annotations.PodNetworkSelectionElements(pod)
	if err != nil {
		return nil, nil, err
	}
	networkStatus, err := annotations.PodDynamicNetworkStatus(pod)
	if err != nil {
		return nil, nil, err
	}

	networkStatusWithoutDefault := []nadv1.NetworkStatus{}

	for i := range networkStatus {
		if networkStatus[i].Default {
			continue
		}
		networkStatusWithoutDefault = append(networkStatusWithoutDefault, networkStatus[i])
	}

	return networkSelectionElements, networkStatusWithoutDefault, err
}
