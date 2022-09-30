package annotations

import (
	"encoding/json"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cni100 "github.com/containernetworking/cni/pkg/types/100"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"
)

var _ = Describe("NetworkStatusFromResponse", func() {
	const (
		ifaceName   = "ens32"
		namespace   = "ns1"
		networkName = "tenantnetwork"
		podName     = "tpod"
	)

	DescribeTable("add dynamic interface to network status", func(initialNetStatus []nadv1.NetworkStatus, resultIPs []string, expectedNetworkStatus string) {
		const (
			ifaceToAdd = "newiface"
			macAddr    = "02:03:04:05:06:07"
		)
		Expect(
			AddDynamicIfaceToStatus(
				newPod(podName, namespace, initialNetStatus...),
				newNetworkSelectionElementWithIface(networkName, ifaceName, namespace),
				newResponse(ifaceToAdd, macAddr, resultIPs...),
			),
		).To(Equal(expectedNetworkStatus))
	},
		Entry("initial empty pod", []nadv1.NetworkStatus{}, nil, `[{"name":"ns1/tenantnetwork","interface":"newiface","mac":"02:03:04:05:06:07","dns":{}}]`),
		Entry("pod with a network present in the network status", []nadv1.NetworkStatus{
			{
				Name:      "net1",
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}},
			nil,
			`[{"name":"net1","interface":"iface1","mac":"00:00:00:20:10:00","dns":{}},{"name":"ns1/tenantnetwork","interface":"newiface","mac":"02:03:04:05:06:07","dns":{}}]`),
		Entry("result with IPs", []nadv1.NetworkStatus{
			{
				Name:      "net1",
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}},
			[]string{"10.10.10.10/24"},
			`[{"name":"net1","interface":"iface1","mac":"00:00:00:20:10:00","dns":{}},{"name":"ns1/tenantnetwork","interface":"newiface","ips":["10.10.10.10"],"mac":"02:03:04:05:06:07","dns":{}}]`))

	DescribeTable("remove an interface to the current network status", func(initialNetStatus []nadv1.NetworkStatus, networkName, ifaceToRemove, expectedNetworkStatus string) {
		Expect(
			DeleteDynamicIfaceFromStatus(
				newPod(podName, namespace, initialNetStatus...),
				newNetworkSelectionElementWithIface(networkName, ifaceToRemove, namespace),
			),
		).To(Equal(expectedNetworkStatus))
	},
		Entry("when there aren't any existing interfaces", nil, "net1", "iface1", "[]"),
		Entry("when we remove all the currently existing interfaces", []nadv1.NetworkStatus{
			{
				Name:      NamespacedName(namespace, networkName),
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}}, networkName, "iface1", "[]"),
		Entry("when there is *not* a matching interface to remove", []nadv1.NetworkStatus{
			{
				Name:      NamespacedName(namespace, networkName),
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			}}, "net2", "iface1", `[{"name":"ns1/tenantnetwork","interface":"iface1","mac":"00:00:00:20:10:00","dns":{}}]`),
		Entry("when we remove one of the existing interfaces", []nadv1.NetworkStatus{
			{
				Name:      NamespacedName(namespace, networkName),
				Interface: "iface1",
				Mac:       "00:00:00:20:10:00",
			},
			{
				Name:      NamespacedName(namespace, "net2"),
				Interface: "iface2",
				Mac:       "aa:bb:cc:20:10:00",
			},
		}, "net2", "iface2", `[{"name":"ns1/tenantnetwork","interface":"iface1","mac":"00:00:00:20:10:00","dns":{}}]`))
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
