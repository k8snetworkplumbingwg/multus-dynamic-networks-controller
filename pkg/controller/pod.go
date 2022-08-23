package controller

import (
	"encoding/json"
	"fmt"
	"reflect"
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

	"github.com/maiqueb/multus-dynamic-networks-controller/pkg/annotations"
	"github.com/maiqueb/multus-dynamic-networks-controller/pkg/logging"
)

const (
	AdvertisedName = "pod-networks-updates"
	maxRetries     = 2
)

type DynamicAttachmentRequestType string

type DynamicAttachmentRequest struct {
	PodName         string
	PodNamespace    string
	AttachmentNames []*nadv1.NetworkSelectionElement
	Type            DynamicAttachmentRequestType
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
	multusSocketPath        string
	nadClientSet            nadclient.Interface
}

// NewPodNetworksController returns new PodNetworksController instance
func NewPodNetworksController(
	k8sCoreInformerFactory v1coreinformerfactory.SharedInformerFactory,
	nadInformers nadinformers.SharedInformerFactory,
	broadcaster record.EventBroadcaster,
	recorder record.EventRecorder,
	multusSocketPath string,
	k8sClientSet kubernetes.Interface,
	nadClientSet nadclient.Interface,
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
		multusSocketPath:        multusSocketPath,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			AdvertisedName),
		k8sClientSet: k8sClientSet,
		nadClientSet: nadClientSet,
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
	if dynamicAttachmentRequest.Type == "add" {
		pod, err := pnc.podsLister.Pods(dynamicAttachmentRequest.PodNamespace).Get(dynamicAttachmentRequest.PodName)
		if err != nil {
			return err
		}
		return pnc.addNetworks(dynamicAttachmentRequest.AttachmentNames, pod)
	} else if dynamicAttachmentRequest.Type == "remove" {
		pod, err := pnc.podsLister.Pods(dynamicAttachmentRequest.PodNamespace).Get(dynamicAttachmentRequest.PodName)
		if err != nil {
			return err
		}
		return pnc.removeNetworks(dynamicAttachmentRequest.AttachmentNames, pod)
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
		klog.Errorf("re-queued request for: %v", dynamicAttachmentRequest)
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

	if reflect.DeepEqual(oldPod.Annotations, newPod.Annotations) {
		return
	}
	podNamespace := oldPod.GetNamespace()
	podName := oldPod.GetName()
	klog.V(logging.Debug).Infof("pod [%s] updated", namespacedName(podNamespace, podName))

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
	klog.Infof("%d attachments to add to pod %s", len(toAdd), namespacedName(podNamespace, podName))
	if len(toAdd) > 0 {
		pnc.workqueue.Add(
			&DynamicAttachmentRequest{
				PodName:         podName,
				PodNamespace:    podNamespace,
				AttachmentNames: toAdd,
				Type:            add,
			})
	}

	toRemove := exclusiveNetworks(oldNetworkSelectionElements, newNetworkSelectionElements)
	klog.Infof("%d attachments to remove from pod %s", len(toRemove), namespacedName(podNamespace, podName))
	if len(toRemove) > 0 {
		pnc.workqueue.Add(
			&DynamicAttachmentRequest{
				PodName:         podName,
				PodNamespace:    podNamespace,
				AttachmentNames: toRemove,
				Type:            remove,
			})
	}
}

func namespacedName(podNamespace string, podName string) string {
	return fmt.Sprintf("%s/%s", podNamespace, podName)
}

func (pnc *PodNetworksController) addNetworks(netsToAdd []*nadv1.NetworkSelectionElement, pod *corev1.Pod) error {
	for i := range netsToAdd {
		klog.Infof("network to add: %v", netsToAdd[i])
		pnc.Eventf(pod, corev1.EventTypeNormal, "AddedInterface", "add network: %s", netsToAdd[i].Name)
	}

	return nil
}

func (pnc *PodNetworksController) removeNetworks(netsToRemove []*nadv1.NetworkSelectionElement, pod *corev1.Pod) error {
	for i := range netsToRemove {
		klog.Infof("network to remove: %v", netsToRemove[i])
		pnc.Eventf(pod, corev1.EventTypeNormal, "RemovedInterface", "removed network: %s", netsToRemove[i].Name)
	}

	return nil
}

func networkSelectionElements(podAnnotations map[string]string, podNamespace string) ([]*nadv1.NetworkSelectionElement, error) {
	podNetworks, ok := podAnnotations[nadv1.NetworkAttachmentAnnot]
	if !ok {
		return nil, fmt.Errorf("the pod is missing the \"%s\" annotation on its annotations: %+v", nadv1.NetworkAttachmentAnnot, podAnnotations)
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
