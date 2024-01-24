package fake

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	crioruntime "k8s.io/cri-api/pkg/apis/runtime/v1"

	criotypes "github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri/crio/types"
)

type CrioClient struct {
	cache map[string]string
}

type ClientOpt func(client *CrioClient)

func NewFakeClient(opts ...ClientOpt) *CrioClient {
	client := &CrioClient{cache: map[string]string{}}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithCachedContainer(containerID string, netnsPath string) ClientOpt {
	return func(client *CrioClient) {
		client.cache[containerID] = netnsPath
	}
}

func (CrioClient) Version(context.Context, *crioruntime.VersionRequest, ...grpc.CallOption) (*crioruntime.VersionResponse, error) {
	return nil, nil
}

func (CrioClient) RunPodSandbox(
	context.Context,
	*crioruntime.RunPodSandboxRequest,
	...grpc.CallOption,
) (*crioruntime.RunPodSandboxResponse, error) {
	return nil, nil
}

func (CrioClient) StopPodSandbox(
	context.Context,
	*crioruntime.StopPodSandboxRequest,
	...grpc.CallOption,
) (*crioruntime.StopPodSandboxResponse, error) {
	return nil, nil
}

func (CrioClient) RemovePodSandbox(
	context.Context,
	*crioruntime.RemovePodSandboxRequest,
	...grpc.CallOption,
) (*crioruntime.RemovePodSandboxResponse, error) {
	return nil, nil
}

func (CrioClient) PodSandboxStatus(
	context.Context,
	*crioruntime.PodSandboxStatusRequest,
	...grpc.CallOption,
) (*crioruntime.PodSandboxStatusResponse, error) {
	return nil, nil
}

func (CrioClient) ListPodSandbox(
	context.Context,
	*crioruntime.ListPodSandboxRequest,
	...grpc.CallOption,
) (*crioruntime.ListPodSandboxResponse, error) {
	return nil, nil
}

func (CrioClient) CreateContainer(
	context.Context,
	*crioruntime.CreateContainerRequest,
	...grpc.CallOption,
) (*crioruntime.CreateContainerResponse, error) {
	return nil, nil
}

func (CrioClient) StartContainer(
	context.Context,
	*crioruntime.StartContainerRequest,
	...grpc.CallOption,
) (*crioruntime.StartContainerResponse, error) {
	return nil, nil
}

func (CrioClient) StopContainer(
	context.Context,
	*crioruntime.StopContainerRequest,
	...grpc.CallOption,
) (*crioruntime.StopContainerResponse, error) {
	return nil, nil
}

func (CrioClient) RemoveContainer(
	context.Context,
	*crioruntime.RemoveContainerRequest,
	...grpc.CallOption,
) (*crioruntime.RemoveContainerResponse, error) {
	return nil, nil
}

func (CrioClient) ListContainers(
	context.Context,
	*crioruntime.ListContainersRequest,
	...grpc.CallOption,
) (*crioruntime.ListContainersResponse, error) {
	return nil, nil
}

func (cc CrioClient) ContainerStatus(
	_ context.Context,
	in *crioruntime.ContainerStatusRequest,
	_ ...grpc.CallOption,
) (*crioruntime.ContainerStatusResponse, error) {
	containerID := in.ContainerId
	netnsPath, wasFound := cc.cache[containerID]
	if !wasFound {
		return nil, fmt.Errorf("container %s not found", containerID)
	}

	containerStatus := newContainerStatusResponseWithLinuxNetworkNamespaceInfo(netnsPath)
	marshalledContainerStatus, err := json.Marshal(&containerStatus)
	if err != nil {
		return nil, fmt.Errorf("error marshaling the container status: %v", err)
	}

	return &crioruntime.ContainerStatusResponse{
		Info: map[string]string{"info": string(marshalledContainerStatus)},
	}, nil
}

func (CrioClient) UpdateContainerResources(
	context.Context,
	*crioruntime.UpdateContainerResourcesRequest,
	...grpc.CallOption,
) (*crioruntime.UpdateContainerResourcesResponse, error) {
	return nil, nil
}

func (CrioClient) ReopenContainerLog(
	context.Context,
	*crioruntime.ReopenContainerLogRequest,
	...grpc.CallOption,
) (*crioruntime.ReopenContainerLogResponse, error) {
	return nil, nil
}

func (CrioClient) ExecSync(
	context.Context,
	*crioruntime.ExecSyncRequest,
	...grpc.CallOption,
) (*crioruntime.ExecSyncResponse, error) {
	return nil, nil
}

func (CrioClient) Exec(
	context.Context,
	*crioruntime.ExecRequest,
	...grpc.CallOption,
) (*crioruntime.ExecResponse, error) {
	return nil, nil
}

func (CrioClient) Attach(
	context.Context,
	*crioruntime.AttachRequest,
	...grpc.CallOption,
) (*crioruntime.AttachResponse, error) {
	return nil, nil
}

func (CrioClient) PortForward(
	context.Context,
	*crioruntime.PortForwardRequest,
	...grpc.CallOption,
) (*crioruntime.PortForwardResponse, error) {
	return nil, nil
}

func (CrioClient) ContainerStats(
	context.Context,
	*crioruntime.ContainerStatsRequest,
	...grpc.CallOption,
) (*crioruntime.ContainerStatsResponse, error) {
	return nil, nil
}

func (CrioClient) ListContainerStats(
	context.Context,
	*crioruntime.ListContainerStatsRequest,
	...grpc.CallOption,
) (*crioruntime.ListContainerStatsResponse, error) {
	return nil, nil
}

func (CrioClient) UpdateRuntimeConfig(
	context.Context,
	*crioruntime.UpdateRuntimeConfigRequest,
	...grpc.CallOption,
) (*crioruntime.UpdateRuntimeConfigResponse, error) {
	return nil, nil
}

func (CrioClient) Status(
	context.Context,
	*crioruntime.StatusRequest,
	...grpc.CallOption,
) (*crioruntime.StatusResponse, error) {
	return nil, nil
}

func (cc CrioClient) PodSandboxStats(
	context.Context,
	*crioruntime.PodSandboxStatsRequest,
	...grpc.CallOption,
) (*crioruntime.PodSandboxStatsResponse, error) {
	return nil, nil
}

func (cc CrioClient) ListPodSandboxStats(
	context.Context,
	*crioruntime.ListPodSandboxStatsRequest,
	...grpc.CallOption,
) (*crioruntime.ListPodSandboxStatsResponse, error) {
	return nil, nil
}

func (cc CrioClient) CheckpointContainer(
	context.Context,
	*crioruntime.CheckpointContainerRequest,
	...grpc.CallOption,
) (*crioruntime.CheckpointContainerResponse, error) {
	return nil, nil
}

func (cc CrioClient) GetContainerEvents(
	context.Context,
	*crioruntime.GetEventsRequest,
	...grpc.CallOption,
) (crioruntime.RuntimeService_GetContainerEventsClient, error) {
	return nil, nil
}

func (cc CrioClient) ListMetricDescriptors(
	context.Context,
	*crioruntime.ListMetricDescriptorsRequest,
	...grpc.CallOption,
) (*crioruntime.ListMetricDescriptorsResponse, error) {
	return nil, nil
}

func (cc CrioClient) ListPodSandboxMetrics(
	context.Context,
	*crioruntime.ListPodSandboxMetricsRequest,
	...grpc.CallOption,
) (*crioruntime.ListPodSandboxMetricsResponse, error) {
	return nil, nil
}

func (cc CrioClient) RuntimeConfig(
	ctx context.Context,
	in *crioruntime.RuntimeConfigRequest,
	opts ...grpc.CallOption,
) (*crioruntime.RuntimeConfigResponse, error) {
	return nil, nil
}

func newContainerStatusResponseWithLinuxNetworkNamespaceInfo(netnsPath string) criotypes.ContainerStatusResponse {
	return criotypes.ContainerStatusResponse{
		RunTimeSpec: criotypes.ContainerRuntimeStatus{
			Linux: criotypes.NamespacesInfo{
				Namespaces: []criotypes.NameSpaceInfo{
					{
						Type: criotypes.ContainerNetworkNamespace,
						Path: netnsPath,
					},
				},
			},
		},
	}
}
