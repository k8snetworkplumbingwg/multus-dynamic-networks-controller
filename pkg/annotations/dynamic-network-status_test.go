package annotations_test

import (
	"encoding/json"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cni100 "github.com/containernetworking/cni/pkg/types/100"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/annotations"
	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/server/api"
)

type attachmentInfo struct {
	ifaceName   string
	networkName string
}

var _ = Describe("NetworkStatusFromResponse", func() {
	const (
		ifaceName   = "ens32"
		ifaceToAdd  = "newiface"
		macAddr     = "02:03:04:05:06:07"
		namespace   = "ns1"
		networkName = "tenantnetwork"
		podName     = "tpod"
	)

	DescribeTable("add dynamic interface to network status", func(initialNetStatus []nadv1.NetworkStatus, resultIPs []string, expectedNetworkStatus []nadv1.NetworkStatus) {
		pod := newPod(podName, namespace, initialNetStatus...)
		currentNetStatus, err := annotations.PodDynamicNetworkStatus(pod)
		Expect(err).NotTo(HaveOccurred())
		Expect(
			annotations.AddDynamicIfaceToStatus(
				currentNetStatus,
				*annotations.NewAttachmentResult(
					newNetworkSelectionElementWithIface(networkName, ifaceName, namespace),
					newResponse(ifaceToAdd, macAddr, resultIPs...),
				),
			),
		).To(Equal(expectedNetworkStatus))
	},
		Entry("initial empty pod", []nadv1.NetworkStatus{}, nil, []nadv1.NetworkStatus{
			{
				Name:      annotations.NamespacedName(namespace, networkName),
				Interface: ifaceToAdd,
				Mac:       macAddr,
				DNS: nadv1.DNS{
					Nameservers: []string{},
					Domain:      "",
					Search:      []string{},
					Options:     []string{},
				},
			}}),
		Entry("pod with a network present in the network status", []nadv1.NetworkStatus{
			{
				Name:      "net1",
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}},
			nil,
			[]nadv1.NetworkStatus{
				{
					Name:      "net1",
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				},
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: ifaceToAdd,
					Mac:       macAddr,
					DNS: nadv1.DNS{
						Nameservers: []string{},
						Domain:      "",
						Search:      []string{},
						Options:     []string{},
					},
				}},
		),
		Entry("result with IPs", []nadv1.NetworkStatus{
			{
				Name:      "net1",
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}},
			[]string{"10.10.10.10/24"},
			[]nadv1.NetworkStatus{
				{
					Name:      "net1",
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				},
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: ifaceToAdd,
					Mac:       macAddr,
					IPs:       []string{"10.10.10.10"},
					DNS: nadv1.DNS{
						Nameservers: []string{},
						Domain:      "",
						Search:      []string{},
						Options:     []string{},
					},
				}},
		))

	DescribeTable("remove an interface to the current network status", func(initialNetStatus []nadv1.NetworkStatus, expectedNetworkStatus []nadv1.NetworkStatus, ifacesToRemove ...attachmentInfo) {
		var netsToRemove []nadv1.NetworkSelectionElement
		for _, ifaceToRemove := range ifacesToRemove {
			netsToRemove = append(
				netsToRemove,
				*newNetworkSelectionElementWithIface(ifaceToRemove.networkName, ifaceToRemove.ifaceName, namespace),
			)
		}
		pod := newPod(podName, namespace, initialNetStatus...)
		currentNetStatus, err := annotations.PodDynamicNetworkStatus(pod)
		Expect(err).NotTo(HaveOccurred())
		Expect(
			annotations.DeleteDynamicIfaceFromStatus(currentNetStatus, netsToRemove...),
		).To(Equal(expectedNetworkStatus))
	},
		Entry("when there aren't any existing interfaces", nil, []nadv1.NetworkStatus{}, attachmentInfo{
			ifaceName:   "iface1",
			networkName: "net1",
		}),
		Entry(
			"when we remove all the currently existing interfaces",
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				}},
			[]nadv1.NetworkStatus{},
			attachmentInfo{
				ifaceName:   "iface1",
				networkName: networkName,
			},
		),
		Entry("when there is *not* a matching interface to remove", []nadv1.NetworkStatus{
			{
				Name:      annotations.NamespacedName(namespace, networkName),
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}},
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				},
			},
			attachmentInfo{
				ifaceName:   "iface1",
				networkName: "net2",
			},
		),
		Entry("when we remove one of the existing interfaces", []nadv1.NetworkStatus{
			{
				Name:      annotations.NamespacedName(namespace, networkName),
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			},
			{
				Name:      annotations.NamespacedName(namespace, "net2"),
				Interface: "iface2",
				Mac:       "aa:bb:cc:20:10:00",
			},
		},
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				},
			},
			attachmentInfo{
				ifaceName:   "iface2",
				networkName: "net2",
			},
		),
		Entry(
			"when we remove multiple interfaces at once",
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
				},
				{
					Name:      annotations.NamespacedName(namespace, "net2"),
					Interface: "iface2",
					Mac:       "aa:bb:cc:20:10:00",
				},
				{
					Name:      annotations.NamespacedName(namespace, "net3"),
					Interface: "iface3",
					Mac:       "aa:bb:cc:11:11:11",
				},
			},
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, "net3"),
					Interface: "iface3",
					Mac:       "aa:bb:cc:11:11:11",
				},
			},
			attachmentInfo{
				ifaceName:   "iface1",
				networkName: networkName,
			},
			attachmentInfo{
				ifaceName:   "iface2",
				networkName: "net2",
			}),
		Entry(
			"the default interface is reported when we delete another interface",
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
					Default:   true,
				},
				{
					Name:      annotations.NamespacedName(namespace, "net2"),
					Interface: "iface2",
					Mac:       "aa:bb:cc:20:10:00",
				},
			},
			[]nadv1.NetworkStatus{
				{
					Name:      annotations.NamespacedName(namespace, networkName),
					Interface: "iface1",
					Mac:       "00:00:00:20:10:00",
					Default:   true,
				},
			},
			attachmentInfo{
				ifaceName:   "iface2",
				networkName: "net2",
			},
		))
})

func newPod(podName string, namespace string, netStatus ...nadv1.NetworkStatus) *corev1.Pod {
	status, err := json.Marshal(netStatus)
	if err != nil {
		return nil
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Annotations: map[string]string{
				nadv1.NetworkStatusAnnot: string(status),
			},
		},
	}
}

func newResponse(ifaceName string, macAddr string, ips ...string) *api.Response {
	var ipConfs []*cni100.IPConfig
	for i := range ips {
		ipConfs = append(ipConfs, &cni100.IPConfig{Address: *ipNet(ips[i])})
	}

	const sandboxPath = "/over/there"
	ifaces := []*cni100.Interface{{
		Name:    ifaceName,
		Mac:     macAddr,
		Sandbox: sandboxPath,
	}}
	return &api.Response{
		Result: &cni100.Result{
			CNIVersion: "1.0.0",
			Interfaces: ifaces,
			IPs:        ipConfs,
		}}
}

func ipNet(ipString string) *net.IPNet {
	ip, network, err := net.ParseCIDR(ipString)
	if err != nil {
		return nil
	}
	network.IP = ip
	return network
}
