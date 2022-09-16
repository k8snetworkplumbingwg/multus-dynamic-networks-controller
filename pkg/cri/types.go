package cri

// RuntimeType indicates the type of runtime
type RuntimeType string

const (
	// Crio represents the CRI-O container runtime
	Crio RuntimeType = "crio"
	// Containerd represents the containerd container runtime
	Containerd RuntimeType = "containerd"
)

// ContainerRuntime interface
type ContainerRuntime interface {
	// NetNS returns the network namespace of the given containerID.
	NetNS(containerID string) (string, error)
}
