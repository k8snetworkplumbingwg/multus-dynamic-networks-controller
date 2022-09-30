package multuscni

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cni100 "github.com/containernetworking/cni/pkg/types/100"

	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dynamic network attachment controller suite")
}

var _ = Describe("multuscni REST client", func() {
	const (
		cniVersion  = "0.4.0"
		networkName = "net1"
		podIP       = "192.168.14.14/24"
		podMAC      = "02:03:04:05:06:07"
	)
	It("errors when the server replies with anything other than 200 OK status", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`kablewit`))
		}))

		defer server.Close()
		_, err := newDummyClient(server.Client(), server.URL).InvokeDelegate(multusRequest())
		Expect(err).To(MatchError("unexpected CNI response status 400: 'kablewit'"))
	})

	It("errors when the service replies with anything other than a `multusapi.Response` structure", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{asd:123}`))
		}))

		defer server.Close()
		_, err := newDummyClient(server.Client(), server.URL).InvokeDelegate(multusRequest())
		Expect(err).To(MatchError(ContainSubstring("failed to unmarshal response '{asd:123}':")))
	})

	DescribeTable("return the expected response", func(response *multusapi.Response) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			serializedResponse, _ := json.Marshal(response)
			_, _ = w.Write(serializedResponse)
		}))

		defer server.Close()
		Expect(newDummyClient(server.Client(), server.URL).InvokeDelegate(multusRequest())).To(Equal(response))
	},
		Entry(
			"when the server replies with a simple L2 CNI result",
			&multusapi.Response{
				Result: &cni100.Result{
					CNIVersion: cniVersion,
					Interfaces: []*cni100.Interface{
						cniInterface(networkName, podMAC),
					},
				}},
		),
		Entry("when the server replies with a CNI result featuring IPs", &multusapi.Response{
			Result: &cni100.Result{
				CNIVersion: cniVersion,
				Interfaces: []*cni100.Interface{
					cniInterface(networkName, podMAC),
				},
				IPs: []*cni100.IPConfig{cniIPConfig(podIP)},
			},
		}),
	)
})

func multusRequest() *multusapi.Request {
	return &multusapi.Request{
		Env:    map[string]string{},
		Config: nil,
	}
}

func newDummyClient(httpClient *http.Client, serverURL string) *HTTPClient {
	return &HTTPClient{httpClient: httpClient, serverURL: serverURL}
}

func cniInterface(networkName, macAddress string) *cni100.Interface {
	return &cni100.Interface{
		Name: networkName,
		Mac:  macAddress,
	}
}

func cniIPConfig(ipStr string) *cni100.IPConfig {
	ip, ipNet, _ := net.ParseCIDR(ipStr)
	ipNet.IP = ip
	return &cni100.IPConfig{
		Address: *ipNet,
	}
}
