package cri

import runtimespec "github.com/opencontainers/runtime-spec/specs-go"

// InfoKey if the key for PodSandboxStatusInfo in the Info map of the PodSandboxStatusResponse
// cri-o: https://github.com/cri-o/cri-o/blob/v1.29.2/server/sandbox_status.go#L114
// containerd: https://github.com/containerd/containerd/blob/v1.7.14/pkg/cri/server/sandbox_status.go#L215
// containerd v2: https://github.com/containerd/containerd/blob/v2.0.0-beta.2/pkg/cri/server/sandbox_status.go#L183
const InfoKey = "info"

// PodSandboxStatusInfo represents the value in the Info map of the PodSandboxStatusResponse with InfoKey as key
// cri-o: https://github.com/cri-o/cri-o/blob/v1.29.2/server/sandbox_status.go#L103
// containerd: https://github.com/containerd/containerd/blob/v1.7.14/pkg/cri/server/sandbox_status.go#L139
// containerd v2: https://github.com/containerd/containerd/blob/v2.0.0-beta.2/pkg/cri/types/sandbox_info.go#L44
type PodSandboxStatusInfo struct {
	RuntimeSpec *runtimespec.Spec `json:"runtimeSpec"`
}
