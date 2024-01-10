package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/e2e/client"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/e2e/status"
	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

func TestDynamicNetworksControllerE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multus Dynamic Networks Controller")
}

var _ = Describe("Multus dynamic networks controller", Serial, func() {
	const (
		namespace   = "ns1"
		networkName = "tenant-network"
		podName     = "tiny-winy-pod"
		timeout     = 15 * time.Second
	)
	var clients *client.E2EClient

	BeforeEach(func() {
		config, err := clusterConfig()
		Expect(err).NotTo(HaveOccurred())

		clients, err = client.New(config)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("a simple network-attachment-definition", func() {
		const initialPodIfaceName = "net1"

		BeforeEach(func() {
			_, err := clients.AddNamespace(namespace)
			Expect(err).NotTo(HaveOccurred())
			_, err = clients.AddNetAttachDef(macvlanNetworkWithoutIPAM(networkName, namespace, lowerDeviceName()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(clients.DeleteNamespace(namespace)).To(Succeed())
		})

		filterPodNonDefaultNetworks := func() []nettypes.NetworkStatus {
			return status.FilterPodsNetworkStatus(clients, namespace, podName, func(networkStatus nettypes.NetworkStatus) bool {
				return !networkStatus.Default
			})
		}

		isTestNamespaceEmpty := func() bool {
			pods, err := clients.ListPods(namespace, appLabel(podName))
			if err != nil {
				return false
			}
			return len(pods.Items) == 0
		}

		Context("a provisioned pod having network selection elements", func() {
			var pod *corev1.Pod

			initialIfaceNetworkStatus := nettypes.NetworkStatus{
				Name:      namespacedName(namespace, networkName),
				Interface: initialPodIfaceName,
			}

			BeforeEach(func() {
				var err error
				pod, err = clients.ProvisionPod(
					podName,
					namespace,
					podAppLabel(podName),
					PodNetworkSelectionElements(
						dynamicNetworkInfo{
							namespace:   namespace,
							networkName: networkName,
							ifaceName:   initialPodIfaceName,
						}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(filterPodNonDefaultNetworks()).Should(
					WithTransform(
						status.CleanMACAddressesFromStatus(),
						ConsistOf(initialIfaceNetworkStatus),
					))
			})

			AfterEach(func() {
				Expect(clients.DeletePod(pod)).To(Succeed())
				Eventually(isTestNamespaceEmpty, timeout).Should(BeTrue())
			})

			It("manages to add a new interface to a running pod", func() {
				const ifaceToAdd = "ens58"

				Expect(clients.AddNetworkToPod(pod, &nettypes.NetworkSelectionElement{
					Name:             networkName,
					Namespace:        namespace,
					InterfaceRequest: ifaceToAdd,
				})).To(Succeed())
				Eventually(filterPodNonDefaultNetworks, timeout).Should(
					WithTransform(
						status.CleanMACAddressesFromStatus(),
						ConsistOf(
							nettypes.NetworkStatus{
								Name:      namespacedName(namespace, networkName),
								Interface: ifaceToAdd,
							},
							initialIfaceNetworkStatus)))
			})

			It("manages to remove an interface from a running pod", func() {
				const ifaceToRemove = initialPodIfaceName

				Expect(clients.RemoveNetworkFromPod(pod, networkName, namespace, ifaceToRemove)).To(Succeed())
				Eventually(filterPodNonDefaultNetworks, timeout).Should(BeEmpty())
			})

			It("interfaces can be added / removed in the same operation", func() {
				const newIface = "eth321"
				Expect(clients.SetPodNetworks(
					pod,
					NetworkSelectionElements(
						dynamicNetworkInfo{
							namespace:   namespace,
							networkName: networkName,
							ifaceName:   newIface,
						})...,
				)).To(Succeed())
				Eventually(filterPodNonDefaultNetworks, timeout).Should(
					WithTransform(
						status.CleanMACAddressesFromStatus(),
						ConsistOf(
							nettypes.NetworkStatus{
								Name:      namespacedName(namespace, networkName),
								Interface: newIface,
							},
						)))
			})

			Context("a network with IPAM", func() {
				const (
					ifaceToAddWithIPAM = "ens202"
					ipAddressToAdd     = "10.10.10.111"
					ipamNetworkToAdd   = "tenant-network-ipam"
					netmaskLen         = 24
				)

				macAddress := "02:03:04:05:06:07"

				BeforeEach(func() {
					_, err := clients.AddNetAttachDef(macvlanNetworkWitStaticIPAM(ipamNetworkToAdd, namespace, lowerDeviceName()))
					Expect(err).NotTo(HaveOccurred())
					Expect(clients.AddNetworkToPod(pod, &nettypes.NetworkSelectionElement{
						Name:             ipamNetworkToAdd,
						Namespace:        namespace,
						IPRequest:        []string{ipWithMask(ipAddressToAdd, netmaskLen)},
						InterfaceRequest: ifaceToAddWithIPAM,
						MacRequest:       macAddress,
					})).To(Succeed())
				})

				It("can be hotplugged into a running pod", func() {
					Eventually(filterPodNonDefaultNetworks, timeout).Should(
						ContainElements(
							nettypes.NetworkStatus{
								Name:      namespacedName(namespace, ipamNetworkToAdd),
								Interface: ifaceToAddWithIPAM,
								IPs:       []string{ipAddressToAdd},
								Mac:       macAddress,
							},
						))
				})

				It("can be hot unplugged from a running pod", func() {
					const ifaceToRemove = ifaceToAddWithIPAM
					pods, err := clients.ListPods(namespace, appLabel(podName))
					Expect(err).NotTo(HaveOccurred())
					pod = &pods.Items[0]

					Expect(clients.RemoveNetworkFromPod(pod, networkName, namespace, ifaceToRemove)).To(Succeed())
					Eventually(filterPodNonDefaultNetworks, timeout).Should(
						Not(ContainElements(
							nettypes.NetworkStatus{
								Name:      namespacedName(namespace, ipamNetworkToAdd),
								Interface: ifaceToAddWithIPAM,
								IPs:       []string{ipAddressToAdd},
								Mac:       macAddress,
							},
						)))
				})
			})
		})

		Context("a provisioned pod featuring *only* the cluster's default network", func() {
			var pod *corev1.Pod

			BeforeEach(func() {
				var err error
				pod, err = clients.ProvisionPod(
					podName,
					namespace,
					podAppLabel(podName),
					PodNetworkSelectionElements())
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(clients.DeletePod(pod)).To(Succeed())
				Eventually(isTestNamespaceEmpty, timeout).Should(BeTrue())
			})

			It("manages to add a new interface to a running pod", func() {
				const (
					desiredMACAddr = "02:03:04:05:06:07"
					ifaceToAdd     = "ens58"
				)

				Expect(clients.AddNetworkToPod(pod, &nettypes.NetworkSelectionElement{
					Name:             networkName,
					Namespace:        namespace,
					InterfaceRequest: ifaceToAdd,
					MacRequest:       desiredMACAddr,
				})).To(Succeed())
				Eventually(filterPodNonDefaultNetworks, timeout).Should(
					ConsistOf(
						nettypes.NetworkStatus{
							Name:      namespacedName(namespace, networkName),
							Interface: ifaceToAdd,
							Mac:       desiredMACAddr,
						}))
			})
		})

		Context("a provisioned pod whose network selection elements do not feature the interface name", func() {
			const (
				ifaceToAdd = "ens58"
				macAddress = "02:03:04:05:06:07"
			)

			var (
				pod                      *corev1.Pod
				initialPodsNetworkStatus []nettypes.NetworkStatus
			)

			runtimePodNetworkStatus := func() []nettypes.NetworkStatus {
				return status.FilterPodsNetworkStatus(
					clients,
					namespace,
					podName,
					func(networkStatus nettypes.NetworkStatus) bool {
						return true
					},
				)
			}

			BeforeEach(func() {
				var err error
				pod, err = clients.ProvisionPod(
					podName,
					namespace,
					podAppLabel(podName),
					PodNetworkSelectionElements(
						dynamicNetworkInfo{
							namespace:   namespace,
							networkName: networkName,
						}),
				)
				Expect(err).NotTo(HaveOccurred())

				initialPodsNetworkStatus = clients.NetworkStatus(pod)
				const defaultNetworkPlusInitialAttachment = 2
				Expect(initialPodsNetworkStatus).To(HaveLen(defaultNetworkPlusInitialAttachment))

				Expect(clients.AddNetworkToPod(pod, &nettypes.NetworkSelectionElement{
					Name:             networkName,
					Namespace:        namespace,
					InterfaceRequest: ifaceToAdd,
					MacRequest:       macAddress,
				})).To(Succeed())

				By("the attachment is ignored since we cannot reconcile without knowing the interface name of all attachments")
				Consistently(runtimePodNetworkStatus, timeout).Should(ConsistOf(initialPodsNetworkStatus))
			})

			AfterEach(func() {
				Expect(clients.DeletePod(pod)).To(Succeed())
				Eventually(isTestNamespaceEmpty, timeout).Should(BeTrue())
			})

			runningPod := func() *corev1.Pod {
				pods, err := clients.ListPods(namespace, appLabel(podName))
				ExpectWithOffset(1, err).NotTo(HaveOccurred())
				ExpectWithOffset(1, pods.Items).NotTo(BeEmpty())
				return &pods.Items[0]
			}

			It("manages to add a new interface to a running pod once the desired state features the interface names", func() {
				By("setting the interface name in the existing attachment")
				Expect(clients.SetInterfaceNamesOnPodsNetworkSelectionElements(runningPod())).To(Succeed())

				Eventually(runtimePodNetworkStatus, 2*timeout).Should( // this test takes longer, for unknown reasons
					ConsistOf(
						append(initialPodsNetworkStatus, nettypes.NetworkStatus{
							Name:      namespacedName(namespace, networkName),
							Interface: ifaceToAdd,
							Mac:       macAddress,
						}),
					))
			})
		})
	})
})

func clusterConfig() (*rest.Config, error) {
	const kubeconfig = "KUBECONFIG"

	kubeconfigPath, found := os.LookupEnv(kubeconfig)
	if !found {
		homePath := os.Getenv("HOME")
		kubeconfigPath = fmt.Sprintf("%s/.kube/config", homePath)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func macvlanNetworkWithoutIPAM(networkName string, namespaceName string, lowerDevice string) *nettypes.NetworkAttachmentDefinition {
	macvlanConfig := fmt.Sprintf(`{
        "cniVersion": "0.3.0",
        "disableCheck": true,
        "plugins": [
            {
                "type": "macvlan",
                "master": "%s",
                "mode": "bridge"
            }
        ]
    }`, lowerDevice)
	return generateNetAttachDefSpec(networkName, namespaceName, macvlanConfig)
}

func macvlanNetworkWitStaticIPAM(networkName string, namespaceName string, lowerDevice string) *nettypes.NetworkAttachmentDefinition {
	macvlanConfig := fmt.Sprintf(`{
        "cniVersion": "0.3.0",
        "disableCheck": true,
        "name": "%s",
        "plugins": [
			{
				"type": "macvlan",
				"capabilities": { "ips": true },
				"master": "%s",
				"mode": "bridge",
				"ipam": {
					"type": "static"
				}
			}, {
				"type": "tuning"
			}
        ]
    }`, networkName, lowerDevice)
	return generateNetAttachDefSpec(networkName, namespaceName, macvlanConfig)
}

func generateNetAttachDefSpec(name, namespace, config string) *nettypes.NetworkAttachmentDefinition {
	return &nettypes.NetworkAttachmentDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "NetworkAttachmentDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: nettypes.NetworkAttachmentDefinitionSpec{
			Config: config,
		},
	}
}

func podAppLabel(appName string) map[string]string {
	const (
		app = "app"
	)

	return map[string]string{app: appName}
}

type dynamicNetworkInfo struct {
	namespace   string
	networkName string
	ifaceName   string
}

func PodNetworkSelectionElements(networkConfig ...dynamicNetworkInfo) map[string]string {
	podNetworkConfig := NetworkSelectionElements(networkConfig...)
	if podNetworkConfig == nil {
		return map[string]string{}
	}
	podNetworksConfig, err := json.Marshal(podNetworkConfig)
	if err != nil {
		return map[string]string{}
	}
	return map[string]string{
		nettypes.NetworkAttachmentAnnot: string(podNetworksConfig),
	}
}

func NetworkSelectionElements(networkConfig ...dynamicNetworkInfo) []nettypes.NetworkSelectionElement {
	var podNetworkConfig []nettypes.NetworkSelectionElement
	for i := range networkConfig {
		podNetworkConfig = append(
			podNetworkConfig,
			nettypes.NetworkSelectionElement{
				Name:             networkConfig[i].networkName,
				Namespace:        networkConfig[i].namespace,
				InterfaceRequest: networkConfig[i].ifaceName,
			},
		)
	}
	return podNetworkConfig
}

func namespacedName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func ipWithMask(ip string, netmaskLen int) string {
	return fmt.Sprintf("%s/%d", ip, netmaskLen)
}

func lowerDeviceName() string {
	const (
		defaultLowerDeviceIfaceName = "eth0"
		lowerDeviceEnvVarKeyName    = "LOWER_DEVICE"
	)

	if lowerDeviceIfaceName, wasFound := os.LookupEnv(lowerDeviceEnvVarKeyName); wasFound {
		return lowerDeviceIfaceName
	}
	return defaultLowerDeviceIfaceName
}

func appLabel(appName string) string {
	return fmt.Sprintf("app=%s", appName)
}
