package annotations

import (
	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"testing"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotation parsing suite")
}

var _ = Describe("Parsing annotations", func() {
	const namespace = "ns1"

	It("nil input", func() {
		_, err := ParsePodNetworkAnnotations("", namespace)
		Expect(err).To(MatchError("parsePodNetworkAnnotation: pod annotation does not have \"network\" as key"))
	})

	It("empty list input", func() {
		Expect(ParsePodNetworkAnnotations("[]", namespace)).To(BeEmpty())
	})

	It("single network name", func() {
		const networkName = "net1"
		Expect(ParsePodNetworkAnnotations(networkName, namespace)).To(ConsistOf(newNetworkSelectionElement(networkName, namespace)))
	})

	It("comma separated list of network names", func() {
		const (
			networkName       = "net1"
			secondNetworkName = "net321"
		)
		Expect(
			ParsePodNetworkAnnotations(networkSelectionElements(networkName, secondNetworkName), namespace),
		).To(
			ConsistOf(
				newNetworkSelectionElement(networkName, namespace),
				newNetworkSelectionElement(secondNetworkName, namespace)))
	})

	It("comma separated list of network names with interface names", func() {
		const (
			networkAndInterfaceNamingPair       = "net1@eth1"
			secondNetworkAndInterfaceNamingPair = "net321@eth2"
		)
		Expect(
			ParsePodNetworkAnnotations(
				networkSelectionElements(
					networkAndInterfaceNamingPair,
					secondNetworkAndInterfaceNamingPair),
				namespace),
		).To(
			ConsistOf(
				newNetworkSelectionElementWithIface("net1", "eth1", namespace),
				newNetworkSelectionElementWithIface("net321", "eth2", namespace)))
	})

	It("network selection element specified in JSON", func() {
		const networkSelectionElementsString = "[\n            { \"name\" : \"macvlan-conf-1\" },\n            { \"name\" : \"macvlan-conf-2\", \"interface\": \"ens4\" }\n    ]"
		Expect(
			ParsePodNetworkAnnotations(networkSelectionElementsString, namespace),
		).To(
			ConsistOf(
				newNetworkSelectionElement("macvlan-conf-1", namespace),
				newNetworkSelectionElementWithIface("macvlan-conf-2", "ens4", namespace)))
	})
})

func networkSelectionElements(networkNames ...string) string {
	return strings.Join(networkNames, ",")
}

func newNetworkSelectionElement(networkName string, namespace string) *v1.NetworkSelectionElement {
	return &v1.NetworkSelectionElement{
		Name:      networkName,
		Namespace: namespace,
	}
}

func newNetworkSelectionElementWithIface(networkName string, ifaceName string, namespace string) *v1.NetworkSelectionElement {
	return &v1.NetworkSelectionElement{
		Name:             networkName,
		Namespace:        namespace,
		InterfaceRequest: ifaceName,
	}
}
