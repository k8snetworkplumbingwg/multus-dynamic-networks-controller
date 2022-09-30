package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/maiqueb/multus-dynamic-networks-controller/e2e/client"
	"github.com/maiqueb/multus-dynamic-networks-controller/e2e/status"
)

func TestDynamicNetworksControllerE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multus Dynamic Networks Controller")
}

var _ = Describe("Multus dynamic networks controller", func() {
	const (
		namespace   = "ns1"
		networkName = "tenant-network"
		podName     = "tiny-winy-pod"
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
			_, err = clients.AddNetAttachDef(macvlanNetworkWithWhereaboutsIPAMNetwork(networkName, namespace))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(clients.DeleteNamespace(namespace)).To(Succeed())
		})

		Context("a provisioned pod", func() {
			var pod *corev1.Pod

			initialIfaceNetworkStatus := nettypes.NetworkStatus{
				Name:      namespacedName(namespace, networkName),
				Interface: initialPodIfaceName,
			}

			filterPodNonDefaultNetworks := func() []nettypes.NetworkStatus {
				return status.FilterPodsNetworkStatus(clients, namespace, podName, func(networkStatus nettypes.NetworkStatus) bool {
					return !networkStatus.Default
				})
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
			})

			It("manages to add a new interface to a running pod", func() {
				const ifaceToAdd = "ens58"

				Expect(clients.AddNetworkToPod(pod, networkName, namespace, ifaceToAdd)).To(Succeed())
				Eventually(filterPodNonDefaultNetworks).Should(
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
				Eventually(filterPodNonDefaultNetworks).Should(BeEmpty())
			})
		})
	})
})

func clusterConfig() (*rest.Config, error) {
	const kubeconfig = "KUBECONFIG"

	kubeconfigPath, found := os.LookupEnv(kubeconfig)
	if !found {
		kubeconfigPath = "$HOME/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func macvlanNetworkWithWhereaboutsIPAMNetwork(networkName string, namespaceName string) *nettypes.NetworkAttachmentDefinition {
	macvlanConfig := `{
        "cniVersion": "0.3.0",
        "disableCheck": true,
        "name": "tenant-network",
        "plugins": [
            {
                "type": "macvlan",
                "master": "eth0",
                "mode": "bridge"
            }
        ]
    }`
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

	podNetworksConfig, err := json.Marshal(podNetworkConfig)
	if err != nil {
		return map[string]string{}
	}
	return map[string]string{
		nettypes.NetworkAttachmentAnnot: string(podNetworksConfig),
	}
}

func namespacedName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
