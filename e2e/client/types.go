package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	netclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/annotations"
)

type E2EClient struct {
	k8sClient          kubernetes.Interface
	netAttachDefClient netclient.K8sCniCncfIoV1Interface
}

func New(config *rest.Config) (*E2EClient, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	netClient, err := netclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &E2EClient{
		k8sClient:          clientSet,
		netAttachDefClient: netClient,
	}, nil
}

func (c *E2EClient) AddNetAttachDef(netattach *nettypes.NetworkAttachmentDefinition) (*nettypes.NetworkAttachmentDefinition, error) {
	return c.netAttachDefClient.NetworkAttachmentDefinitions(netattach.ObjectMeta.Namespace).Create(context.TODO(), netattach, metav1.CreateOptions{})
}

func (c *E2EClient) DelNetAttachDef(namespace string, podName string) error {
	return c.netAttachDefClient.NetworkAttachmentDefinitions(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
}

func (c *E2EClient) AddNamespace(name string) (*corev1.Namespace, error) {
	return c.k8sClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, metav1.CreateOptions{})
}

func (c *E2EClient) DeleteNamespace(name string) error {
	const timeout = 30 * time.Second

	if err := c.k8sClient.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		if _, err := c.k8sClient.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{}); err != nil && k8serrors.IsNotFound(err) {
			return true, nil
		} else if err != nil {
			return false, err
		}
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}

func (c *E2EClient) ProvisionPod(podName string, namespace string, label, podAnnotations map[string]string) (*corev1.Pod, error) {
	pod := PodObject(podName, namespace, label, podAnnotations)
	pod, err := c.k8sClient.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	const podCreateTimeout = 10 * time.Second
	if err := c.WaitForPodReady(pod.Namespace, pod.Name, podCreateTimeout); err != nil {
		return nil, err
	}

	pod, err = c.k8sClient.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (c *E2EClient) DeletePod(pod *corev1.Pod) error {
	if err := c.k8sClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	const podDeleteTimeout = 20 * time.Second
	if err := c.WaitForPodToDisappear(pod.GetNamespace(), pod.GetName(), podDeleteTimeout); err != nil {
		return err
	}
	return nil
}

func (c *E2EClient) AddNetworkToPod(pod *corev1.Pod, ifaceConfigsToAdd ...*nettypes.NetworkSelectionElement) error {
	pod.ObjectMeta.Annotations[nettypes.NetworkAttachmentAnnot] = dynamicNetworksAnnotation(pod, ifaceConfigsToAdd...)
	_, err := c.k8sClient.CoreV1().Pods(pod.GetNamespace()).Update(context.TODO(), pod, metav1.UpdateOptions{})
	return err
}

func (c *E2EClient) RemoveNetworkFromPod(pod *corev1.Pod, networkName string, namespace string, ifaceToRemove string) error {
	pod.ObjectMeta.Annotations[nettypes.NetworkAttachmentAnnot] = removeFromDynamicNetworksAnnotation(pod, networkName, namespace, ifaceToRemove)
	_, err := c.k8sClient.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	return err
}

// WaitForPodReady polls up to timeout seconds for pod to enter steady state (running or succeeded state).
// Returns an error if the pod never enters a steady state.
func (c *E2EClient) WaitForPodReady(namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, isPodRunning(c.k8sClient, podName, namespace))
}

// WaitForPodToDisappear polls up to timeout seconds for pod to be gone from the Kubernetes cluster.
// Returns an error if the pod is never deleted, or if GETing it returns an error other than `NotFound`.
func (c *E2EClient) WaitForPodToDisappear(namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, isPodGone(c.k8sClient, podName, namespace))
}

// WaitForPodBySelector waits up to timeout seconds for all pods in 'namespace' with given 'selector' to enter provided state
// If no pods are found, return nil.
func (c *E2EClient) WaitForPodBySelector(namespace, selector string, timeout time.Duration) error {
	podList, err := c.ListPods(namespace, selector)
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return nil
	}

	pods := podList.Items
	for i := range pods {
		if err := c.WaitForPodReady(namespace, pods[i].Name, timeout); err != nil {
			return err
		}
	}
	return nil
}

// ListPods returns the list of currently scheduled or running pods in `namespace` with the given selector
func (c *E2EClient) ListPods(namespace, selector string) (*corev1.PodList, error) {
	listOptions := metav1.ListOptions{LabelSelector: selector}
	podList, err := c.k8sClient.CoreV1().Pods(namespace).List(context.Background(), listOptions)

	if err != nil {
		return nil, err
	}
	return podList, nil
}

func isPodRunning(cs kubernetes.Interface, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := cs.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed:
			return false, errors.New("pod failed")
		case corev1.PodSucceeded:
			return false, errors.New("pod succeeded")
		}

		return false, nil
	}
}

func isPodGone(cs kubernetes.Interface, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := cs.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			return true, nil
		} else if err != nil {
			return false, fmt.Errorf("something weird happened with the pod, which is in state: [%s]. Errors: %w", pod.Status.Phase, err)
		}

		return false, nil
	}
}

func PodObject(podName string, namespace string, label, podAnnotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: podMeta(podName, namespace, label, podAnnotations),
		Spec:       podSpec("samplepod"),
	}
}

func podSpec(containerName string) corev1.PodSpec {
	const testImage = "k8s.gcr.io/e2e-test-images/agnhost:2.26"
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    containerName,
				Command: containerCmd(),
				Image:   testImage,
			},
		},
	}
}

func containerCmd() []string {
	return []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"}
}

func podMeta(podName string, namespace string, label map[string]string, podAnnotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        podName,
		Namespace:   namespace,
		Labels:      label,
		Annotations: podAnnotations,
	}
}

func dynamicNetworksAnnotation(pod *corev1.Pod, newIfaceConfigs ...*nettypes.NetworkSelectionElement) string {
	networkSelectionElements, err := extractPodNetworkSelectionElements(pod)
	if err != nil {
		return ""
	}

	networkSelectionElements = append(
		networkSelectionElements,
		newIfaceConfigs...,
	)
	updatedNetworkSelectionElements, err := json.Marshal(networkSelectionElements)
	if err != nil {
		return ""
	}

	return string(updatedNetworkSelectionElements)
}

func removeFromDynamicNetworksAnnotation(pod *corev1.Pod, networkName string, netNamespace string, ifaceName string) string {
	currentNetworkSelectionElementsString, wasFound := pod.ObjectMeta.Annotations[nettypes.NetworkAttachmentAnnot]
	if !wasFound {
		return ""
	}

	currentNetworkSelectionElements, err := annotations.ParsePodNetworkAnnotations(currentNetworkSelectionElementsString, netNamespace)
	if err != nil {
		return ""
	}

	var updatedNetworkSelectionElements []nettypes.NetworkSelectionElement
	for i := range currentNetworkSelectionElements {
		if currentNetworkSelectionElements[i].Name == networkName && currentNetworkSelectionElements[i].Namespace == netNamespace && currentNetworkSelectionElements[i].InterfaceRequest == ifaceName {
			continue
		}
		updatedNetworkSelectionElements = append(updatedNetworkSelectionElements, *currentNetworkSelectionElements[i])
	}

	var newSelectionElements string
	if len(updatedNetworkSelectionElements) > 0 {
		newSelectionElementsBytes, err := json.Marshal(updatedNetworkSelectionElements)
		if err != nil {
			return ""
		}
		newSelectionElements = string(newSelectionElementsBytes)
	} else {
		newSelectionElements = "[]"
	}
	return newSelectionElements
}

func extractPodNetworkSelectionElements(pod *corev1.Pod) ([]*nettypes.NetworkSelectionElement, error) {
	currentNetworkSelectionElementsString, wasFound := pod.ObjectMeta.Annotations[nettypes.NetworkAttachmentAnnot]
	if !wasFound {
		return []*nettypes.NetworkSelectionElement{}, nil
	}

	currentNetworkSelectionElements, err := annotations.ParsePodNetworkAnnotations(currentNetworkSelectionElementsString, pod.GetNamespace())
	if err != nil {
		return nil, err
	}

	return currentNetworkSelectionElements, nil
}
