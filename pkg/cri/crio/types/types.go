package types

const ContainerNetworkNamespace = "network"

// ContainerStatusResponse represents the container status reply - crictl ps <containerID>
type ContainerStatusResponse struct {
	SandboxID   string
	RunTimeSpec ContainerRuntimeStatus
}

// ContainerRuntimeStatus represents the relevant part of the container status spec
// For now, let's look at linux only.
type ContainerRuntimeStatus struct {
	Linux NamespacesInfo
}

// NamespacesInfo represents the container status namespaces
type NamespacesInfo struct {
	Namespaces []NameSpaceInfo
}

// NameSpaceInfo represents the ns info
type NameSpaceInfo struct {
	Type string
	Path string
}
