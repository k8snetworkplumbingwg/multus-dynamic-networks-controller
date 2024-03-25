package cri_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri/fake"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dynamic network attachment controller suite")
}

var _ = Describe("CRI runtime", func() {
	var runtime *cri.Runtime

	When("the runtime *does not* feature any pod", func() {
		const (
			podUID = "abc-def"
		)

		BeforeEach(func() {
			runtime = newDummyCrioRuntime()
		})

		It("cannot extract the network namespace of a pod", func() {
			_, err := runtime.NetworkNamespace(context.Background(), podUID)
			Expect(err).To(HaveOccurred())
		})

		It("cannot extract the PodSandboxID of a pod", func() {
			_, err := runtime.PodSandboxID(context.Background(), podUID)
			Expect(err).To(HaveOccurred())
		})
	})

	When("a live container is provisioned in the runtime", func() {
		const (
			podUID       = "abc-def"
			podSandboxID = "1234"
			netnsPath    = "bottom-drawer"
		)
		BeforeEach(func() {
			runtime = newDummyCrioRuntime(fake.WithCachedContainer(podUID, podSandboxID, netnsPath))
		})

		It("cannot extract the network namespace of a pod", func() {
			Expect(runtime.NetworkNamespace(context.Background(), podUID)).To(Equal(netnsPath))
		})

		It("cannot extract the PodSandboxID of a pod", func() {
			Expect(runtime.PodSandboxID(context.Background(), podUID)).To(Equal(podSandboxID))
		})
	})
})

func newDummyCrioRuntime(opts ...fake.ClientOpt) *cri.Runtime {
	runtimeClient := fake.NewFakeClient()

	for _, opt := range opts {
		opt(runtimeClient)
	}

	return &cri.Runtime{
		Client: runtimeClient,
	}
}
