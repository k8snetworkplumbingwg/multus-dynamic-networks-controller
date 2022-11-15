package crio

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri/crio/fake"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dynamic network attachment controller suite")
}

var _ = Describe("CRI-O runtime", func() {
	var runtime *Runtime

	When("the runtime *does not* feature any containers", func() {
		BeforeEach(func() {
			runtime = newDummyCrioRuntime()
		})

		It("cannot extract the network namespace of a container", func() {
			_, err := runtime.NetNS("1234")
			Expect(err).To(MatchError("failed to get pod sandbox info: container 1234 not found"))
		})
	})

	When("a live container is provisioned in the runtime", func() {
		const (
			containerID = "1234"
			netnsPath   = "bottom-drawer"
		)
		BeforeEach(func() {
			runtime = newDummyCrioRuntime(fake.WithCachedContainer(containerID, netnsPath))
		})

		It("cannot extract the network namespace of a container", func() {
			Expect(runtime.NetNS(containerID)).To(Equal(netnsPath))
		})
	})
})

func newDummyCrioRuntime(opts ...fake.ClientOpt) *Runtime {
	runtimeClient := fake.NewFakeClient()

	for _, opt := range opts {
		opt(runtimeClient)
	}

	const arbitraryTimeout = 5 * time.Second
	return &Runtime{
		client:         runtimeClient,
		runtimeTimeout: arbitraryTimeout,
	}
}
