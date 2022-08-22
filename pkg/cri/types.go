package cri

// RuntimeType indicates the type of runtime
type RuntimeType string

const (
	// Crio represents the CRI-O container runtime
	Crio RuntimeType = "crio"
	// Containerd represents the containerd container runtime
	Containerd = "containerd"
)
