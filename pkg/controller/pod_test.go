package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cni100 "github.com/containernetworking/cni/pkg/types/100"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1coreinformerfactory "k8s.io/client-go/informers"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	nad "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	fakenadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/fake"
	nadinformers "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/informers/externalversions"
	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/server/api"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/annotations"
	fakecri "github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri/fake"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/multuscni"
	fakemultusclient "github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/multuscni/fake"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dynamic network attachment controller suite")
}

var _ = Describe("Dynamic Attachment controller", func() {
	Context("with access to a proper multus configuration", func() {
		var cniConfigDir string

		BeforeEach(func() {
			const (
				configFilePermissions = 0755
				multusConfigPath      = "00-multus.conf"
			)

			var err error
			cniConfigDir, err = os.MkdirTemp("", "multus-config")
			Expect(err).ToNot(HaveOccurred())
			Expect(os.MkdirAll(cniConfigDir, configFilePermissions)).To(Succeed())
			Expect(os.WriteFile(
				path.Join(cniConfigDir, multusConfigPath),
				[]byte(dummyMultusConfig()), configFilePermissions)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(cniConfigDir)).To(Succeed())
		})

		Context("with an existing running pod", func() {
			const (
				cniVersion  = "0.3.0"
				ipAddr      = "172.16.0.1"
				macAddr     = "02:03:04:05:06:07"
				namespace   = "default"
				networkName = "tiny-net"
				podName     = "tiny-winy-pod"
			)
			cniArgs := &map[string]string{"foo": "bar"}
			var (
				eventRecorder *record.FakeRecorder
				k8sClient     k8sclient.Interface
				pod           *corev1.Pod
				networkToAdd  string
				stopChannel   chan struct{}
				nadClient     nadclient.Interface
			)

			networkStatusNames := func(statuses []nad.NetworkStatus) []string {
				var names []string
				for _, status := range statuses {
					names = append(names, status.Name)
				}
				return names
			}

			BeforeEach(func() {
				pod = podSpec(podName, namespace, networkName)
				networkToAdd = fmt.Sprintf("%s-2", networkName)

				var err error
				nadClient, err = newFakeNetAttachDefClient(
					netAttachDef(networkName, namespace, dummyNetSpec(networkName, cniVersion)),
					netAttachDef(networkToAdd, namespace, dummyNetSpec(networkToAdd, cniVersion)))
				Expect(err).NotTo(HaveOccurred())
				stopChannel = make(chan struct{})
				const maxEvents = 5
				eventRecorder = record.NewFakeRecorder(maxEvents)
			})

			AfterEach(func() {
				close(stopChannel)
			})

			JustBeforeEach(func() {
				k8sClient = fake.NewSimpleClientset(pod)
				Expect(
					newDummyPodController(
						k8sClient,
						nadClient,
						stopChannel,
						eventRecorder,
						fakecri.NewFakeRuntime(*pod),
						fakemultusclient.NewFakeClient(
							networkConfig(multuscni.CmdAdd, "net1", macAddr),
							networkConfig(multuscni.CmdDel, "net0", "")),
					)).NotTo(BeNil())
				Expect(func() []nad.NetworkStatus {
					updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
					if err != nil {
						return nil
					}
					status, err := annotations.PodDynamicNetworkStatus(updatedPod)
					if err != nil {
						return nil
					}
					return status
				}()).Should(
					And(
						WithTransform(networkStatusNames, ContainElements(annotations.NamespacedName(namespace, networkName))),
						Not(WithTransform(networkStatusNames, ContainElements(annotations.NamespacedName(namespace, networkToAdd))))),
				)
			})

			When("an attachment is added to the pod's network annotations", func() {
				JustBeforeEach(func() {
					var err error
					_, err = k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod, networkName, networkToAdd),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `AddedInterface` event is seen in the event recorded ", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal AddedInterface pod [%s]: added interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})

				It("the pod network-status is updated with the new network attachment", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", ""),
						ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})
			})

			When("an attachment is removed from the pod's network annotations", func() {
				JustBeforeEach(func() {
					var err error
					_, err = k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `RemovedInterface` event is seen in the event recorded ", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal RemovedInterface pod [%s]: removed interface %s from network: %s",
						annotations.NamespacedName(namespace, podName),
						"net0",
						networkName,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})

				It("the pod network-status no longer features the removed network", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(BeEmpty())
				})
			})

			When("an attachment is added to a host networked pod", func() {
				BeforeEach(func() {
					pod = hostNetworkedPodSpec(podName, namespace, networkName)
				})

				JustAfterEach(func() {
					_, err := k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod, networkName, networkToAdd),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("the attachment request is ignored", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", "")))
				})

				It("throws an event indicating the interface add operation is rejected", func() {
					expectedEventPayload := fmt.Sprintf(
						"Warning InterfaceAddRejected pod [%s]: will not add interface to host networked pod",
						annotations.NamespacedName(namespace, podName),
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})
			})

			When("an attachment is added with attributes (IPs, MAC, cni-args)", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					netSelectionElements := append(generateNetworkSelectionElements(namespace, networkName),
						nad.NetworkSelectionElement{
							Name:             networkToAdd,
							Namespace:        namespace,
							InterfaceRequest: "net1",
							IPRequest:        []string{ipAddr},
							MacRequest:       macAddr,
							CNIArgs: &map[string]interface{}{
								"foo": "bar",
							},
						},
					)
					serelizedNetSelectionElements, _ := json.Marshal(netSelectionElements)
					pod.Annotations[nad.NetworkAttachmentAnnot] = string(serelizedNetSelectionElements)
					_, err := k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						pod,
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `AddedInterface` event is seen in the event recorded", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal AddedInterface pod [%s]: added interface %s to network: %s(ips: [%s], mac: %s, cni-args: %v)",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
						ipAddr,
						macAddr,
						cniArgs,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})

				It("the pod network-status is updated with the new network attachment", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", ""),
						ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})
			})

			When("an attachment is removed and another is added from the pod's network annotations", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					netSelectionElements := []nad.NetworkSelectionElement{
						{
							Name:             networkToAdd,
							Namespace:        namespace,
							InterfaceRequest: "net1",
						},
					}
					serelizedNetSelectionElements, _ := json.Marshal(netSelectionElements)
					pod.Annotations[nad.NetworkAttachmentAnnot] = string(serelizedNetSelectionElements)
					var err error
					_, err = k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						pod,
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `AddedInterface` event and then and `RemovedInterface` event are seen in the event recorded", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal AddedInterface pod [%s]: added interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
					expectedEventPayload = fmt.Sprintf(
						"Normal RemovedInterface pod [%s]: removed interface %s from network: %s",
						annotations.NamespacedName(namespace, podName),
						"net0",
						networkName,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})

				It("the pod network-status is updated with the new network attachment and without the other one", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})
			})

			When("a wrong attachment is added to the pod's network annotations with a following correct attachement", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					netSelectionElements := append(generateNetworkSelectionElements(namespace, networkName),
						[]nad.NetworkSelectionElement{
							{
								Name:             networkToAdd,
								Namespace:        namespace,
								InterfaceRequest: "net-non-existing",
							},
							{
								Name:             networkToAdd,
								Namespace:        namespace,
								InterfaceRequest: "net1",
							},
						}...,
					)
					serelizedNetSelectionElements, _ := json.Marshal(netSelectionElements)
					pod.Annotations[nad.NetworkAttachmentAnnot] = string(serelizedNetSelectionElements)
					var err error
					_, err = k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						pod,
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `FailedAddingInterface` event is seen, no `AddedInterface` event is seen and no changes the status", func() {
					expectedEventPayload := fmt.Sprintf(
						"Warning FailedAddingInterface pod [%s]: failed adding interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net-non-existing",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))

					// reconciliation requeued without adding the next interface (net1).
					expectedEventPayload = fmt.Sprintf(
						"Warning FailedAddingInterface pod [%s]: failed adding interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net-non-existing",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))

					// This is not in a separate "It" since there is no change and it should wait for the events
					// No pod network-status is added since the first one failed.
					Consistently(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", "")))
				})
			})

			When("a wrong attachment is added to the pod's network annotations after a correct attachement", func() {
				JustBeforeEach(func() {
					var err error
					pod = updatePodSpec(pod)
					netSelectionElements := append(generateNetworkSelectionElements(namespace, networkName),
						[]nad.NetworkSelectionElement{
							{
								Name:             networkToAdd,
								Namespace:        namespace,
								InterfaceRequest: "net1",
							},
							{
								Name:             networkToAdd,
								Namespace:        namespace,
								InterfaceRequest: "net-non-existing",
							},
						}...,
					)
					serelizedNetSelectionElements, _ := json.Marshal(netSelectionElements)
					pod.Annotations[nad.NetworkAttachmentAnnot] = string(serelizedNetSelectionElements)
					_, err = k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						pod,
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("an `AddedInterface` event is seen, followed by a `FailedAddingInterface` event", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal AddedInterface pod [%s]: added interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))

					expectedEventPayload = fmt.Sprintf(
						"Warning FailedAddingInterface pod [%s]: failed adding interface %s to network: %s",
						annotations.NamespacedName(namespace, podName),
						"net-non-existing",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})

				It("the pod network-status is updated with only the first added network attachment", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", ""),
						ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})
			})

			When("an attachment is removed followed by a failing removal of another attachment", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					pod.Annotations[nad.NetworkStatusAnnot] =
						`[
							{
								"name": "default/tiny-net",
								"interface": "net0",
								"dns": {}
							},
							{
								"name": "default/tiny-net-2",
								"interface": "net1",
								"mac": "02:03:04:05:06:07",
								"dns": {}
							}
						]`
					_, err := k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("the pod network-status is updated with only the first network status removed", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})

				It("a `RemovedInterface` event is seen followed by a `FailedRemovingInterface` event", func() {
					expectedEventPayload := fmt.Sprintf(
						"Normal RemovedInterface pod [%s]: removed interface %s from network: %s",
						annotations.NamespacedName(namespace, podName),
						"net0",
						networkName,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))

					expectedEventPayload = fmt.Sprintf(
						"Warning FailedRemovingInterface pod [%s]: failed removing interface %s from network: %s",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})
			})

			When("an attachment is removed after a failing removal of another attachment", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					pod.Annotations[nad.NetworkStatusAnnot] =
						`[
							{
								"name": "default/tiny-net-2",
								"interface": "net1",
								"mac": "02:03:04:05:06:07",
								"dns": {}
							},
							{
								"name": "default/tiny-net",
								"interface": "net0",
								"dns": {}
							}
						]`
					_, err := k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("the pod network-status has not changed", func() {
					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(
						ifaceStatusForDefaultNamespace(networkName, "net0", ""),
						ifaceStatusForDefaultNamespace(networkToAdd, "net1", macAddr)))
				})

				It("a `RemovedInterface` event is seen", func() {
					expectedEventPayload := fmt.Sprintf(
						"Warning FailedRemovingInterface pod [%s]: failed removing interface %s from network: %s",
						annotations.NamespacedName(namespace, podName),
						"net1",
						networkToAdd,
					)
					Eventually(<-eventRecorder.Events).Should(Equal(expectedEventPayload))
				})
			})

			When("an attachment status for the default network is added", func() {
				JustBeforeEach(func() {
					pod = updatePodSpec(pod)
					pod.Annotations[nad.NetworkStatusAnnot] = `[{"name":"default/tiny-net","default": true,"interface":"net0","dns":{}}]`
					_, err := k8sClient.CoreV1().Pods(namespace).UpdateStatus(
						context.TODO(),
						updatePodSpec(pod),
						metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("the pod network-status has not changed (default network status must be ignored)", func() {
					defaultNet := ifaceStatusForDefaultNamespace(networkName, "net0", "")
					defaultNet.Default = true

					Eventually(func() ([]nad.NetworkStatus, error) {
						updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						status, err := annotations.PodDynamicNetworkStatus(updatedPod)
						if err != nil {
							return nil, err
						}
						return status, nil
					}).Should(ConsistOf(defaultNet))
				})
			})

		})
	})
})

func networkConfig(cmd, ifaceName, mac string) fakemultusclient.NetworkConfig {
	const cniVersion = "1.0.0"
	return fakemultusclient.NetworkConfig{
		Cmd:       cmd,
		IfaceName: ifaceName,
		Response: &multusapi.Response{
			Result: &cni100.Result{
				CNIVersion: cniVersion,
				Interfaces: []*cni100.Interface{
					{Name: ifaceName, Mac: mac, Sandbox: "asd"},
				},
			}},
	}
}

type dummyPodController struct {
	*PodNetworksController
	networkCache cache.Store
	podCache     cache.Store
}

func newDummyPodController(
	k8sClient k8sclient.Interface,
	nadClient nadclient.Interface,
	stopChannel chan struct{},
	recorder record.EventRecorder,
	containerRuntime ContainerRuntime,
	multusClient multuscni.Client) (*dummyPodController, error) {
	const noResyncPeriod = 0
	netAttachDefInformerFactory := nadinformers.NewSharedInformerFactory(nadClient, noResyncPeriod)
	podInformerFactory := v1coreinformerfactory.NewSharedInformerFactory(k8sClient, noResyncPeriod)

	podController, _ := NewPodNetworksController(
		podInformerFactory,
		netAttachDefInformerFactory,
		nil,
		recorder,
		k8sClient,
		nadClient,
		containerRuntime,
		multusClient)

	alwaysReady := func() bool { return true }
	podController.arePodsSynched = alwaysReady
	podController.areNetAttachDefsSynched = alwaysReady

	podInformerFactory.Start(stopChannel)
	synced := podInformerFactory.WaitForCacheSync(stopChannel)
	for v, ok := range synced {
		if !ok {
			fmt.Fprintf(os.Stderr, "caches failed to sync (podInformerFactory): %v", v)
		}
	}
	netAttachDefInformerFactory.Start(stopChannel)
	synced = netAttachDefInformerFactory.WaitForCacheSync(stopChannel)
	for v, ok := range synced {
		if !ok {
			fmt.Fprintf(os.Stderr, "caches failed to sync (netAttachDefInformerFactory): %v", v)
		}
	}

	controller := &dummyPodController{
		PodNetworksController: podController,
		networkCache:          podController.netAttachDefInformer.GetStore(),
		podCache:              podController.podsInformer.GetStore(),
	}

	if err := controller.initControllerCaches(k8sClient, nadClient); err != nil {
		return nil, err
	}
	go podController.Start(stopChannel)

	return controller, nil
}

func newFakeNetAttachDefClient(networkAttachments ...nad.NetworkAttachmentDefinition) (nadclient.Interface, error) {
	netAttachDefClient := fakenadclient.NewSimpleClientset()
	gvr := metav1.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}

	for i := range networkAttachments {
		if err := netAttachDefClient.Tracker().Create(
			schema.GroupVersionResource(gvr),
			&networkAttachments[i],
			networkAttachments[i].GetNamespace()); err != nil {
			return nil, err
		}
	}
	return netAttachDefClient, nil
}

func (dpc *dummyPodController) initControllerCaches(k8sClient k8sclient.Interface, nadClient nadclient.Interface) error {
	if err := dpc.synchPods(k8sClient); err != nil {
		return err
	}
	if err := dpc.synchNetworkAttachments(nadClient); err != nil {
		return err
	}
	return nil
}

func (dpc *dummyPodController) synchNetworkAttachments(netAttachDefClient nadclient.Interface) error {
	const allNamespaces = ""

	networkAttachments, err := netAttachDefClient.K8sCniCncfIoV1().NetworkAttachmentDefinitions(allNamespaces).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range networkAttachments.Items {
		if err := dpc.networkCache.Add(&networkAttachments.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (dpc *dummyPodController) synchPods(k8sClient k8sclient.Interface) error {
	const allNamespaces = ""

	pods, err := k8sClient.CoreV1().Pods(allNamespaces).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range pods.Items {
		if err := dpc.podCache.Add(&pods.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func dummyNetSpec(networkName string, cniVersion string) string {
	return fmt.Sprintf(`{
      "cniVersion": "%s",
      "name": "%s",
      "type": "macvlan",
      "master": "eth0",
      "mode": "bridge"
    }`, cniVersion, networkName)
}

func podSpec(name string, namespace string, networks ...string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: podNetworkConfig(networks...),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					ContainerID: name,
				},
			},
		},
	}
}

func netAttachDef(netName string, namespace string, config string) nad.NetworkAttachmentDefinition {
	return nad.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      netName,
			Namespace: namespace,
		},
		Spec: nad.NetworkAttachmentDefinitionSpec{
			Config: config,
		},
	}
}

func updatePodSpec(pod *corev1.Pod, networkNames ...string) *corev1.Pod {
	newPod := pod.DeepCopy()
	newPod.Annotations[nad.NetworkAttachmentAnnot] = generateNetworkSelectionAnnotation(
		"default", networkNames...)
	return newPod
}

// this should be used when "creating" a new pod - it sets the status.
func podNetworkConfig(networkNames ...string) map[string]string {
	return map[string]string{
		nad.NetworkAttachmentAnnot: generateNetworkSelectionAnnotation("default", networkNames...),
		nad.NetworkStatusAnnot:     podNetworkStatusAnnotations("default", networkNames...),
	}
}

func generateNetworkSelectionElements(namespace string, networkNames ...string) []nad.NetworkSelectionElement {
	var netSelectionElements []nad.NetworkSelectionElement
	for i, networkName := range networkNames {
		netSelectionElements = append(
			netSelectionElements,
			nad.NetworkSelectionElement{
				Name:             networkName,
				Namespace:        namespace,
				InterfaceRequest: fmt.Sprintf("net%d", i),
			})
	}
	if netSelectionElements == nil {
		netSelectionElements = make([]nad.NetworkSelectionElement, 0)
	}
	return netSelectionElements
}

func generateNetworkSelectionAnnotation(namespace string, networkNames ...string) string {
	netSelectionElements := generateNetworkSelectionElements(namespace, networkNames...)
	serelizedNetSelectionElements, err := json.Marshal(netSelectionElements)
	if err != nil {
		return ""
	}
	return string(serelizedNetSelectionElements)
}

func podNetworkStatusAnnotations(namespace string, networkNames ...string) string {
	var netStatus []nad.NetworkStatus
	for i, networkName := range networkNames {
		netStatus = append(
			netStatus,
			nad.NetworkStatus{
				Name:      fmt.Sprintf("%s/%s", namespace, networkName),
				Interface: fmt.Sprintf("net%d", i),
			})
	}
	serelizedNetStatus, err := json.Marshal(netStatus)
	if err != nil {
		return ""
	}
	return string(serelizedNetStatus)
}

func dummyMultusConfig() string {
	return `{
    "name": "node-cni-network",
    "type": "multus",
    "kubeconfig": "/etc/kubernetes/node-kubeconfig.yaml",
    "delegates": [{
        "type": "weave-net"
    }],
	"runtimeConfig": {
      "portMappings": [
        {"hostPort": 8080, "containerPort": 80, "protocol": "tcp"}
      ]
    }
}`
}

func ifaceStatusForDefaultNamespace(networkName, ifaceName, macAddress string) nad.NetworkStatus {
	const namespace = "default"
	return nad.NetworkStatus{
		Name:      fmt.Sprintf("%s/%s", namespace, networkName),
		Interface: ifaceName,
		Mac:       macAddress,
	}
}

func hostNetworkedPodSpec(name string, namespace string, networks ...string) *corev1.Pod {
	pod := podSpec(name, namespace, networks...)
	pod.Spec.HostNetwork = true
	return pod
}
