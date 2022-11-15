package crio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	crioruntime "k8s.io/cri-api/pkg/apis/runtime/v1"

	criotypes "github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri/crio/types"
)

// Runtime represents a connection to the CRI-O runtime
type Runtime struct {
	client         crioruntime.RuntimeServiceClient
	runtimeTimeout time.Duration
}

// NewRuntime returns a connection to the CRI-O runtime
func NewRuntime(socketPath string, timeout time.Duration) (*Runtime, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("path to cri-o socket missing")
	}

	clientConnection, err := connect(socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("error establishing connection to CRI-O: %w", err)
	}

	return &Runtime{
		client:         crioruntime.NewRuntimeServiceClient(clientConnection),
		runtimeTimeout: timeout,
	}, nil
}

func connect(socketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("endpoint is not set")
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()
	conn, err := grpc.DialContext(
		ctx,
		criServerAddress(socketPath),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to endpoint '%s': %v", socketPath, err)
	}
	return conn, nil
}

// NetNS returns the network namespace of the given containerID.
func (cr *Runtime) NetNS(containerID string) (string, error) {
	reply, err := cr.client.ContainerStatus(context.Background(), &crioruntime.ContainerStatusRequest{
		ContainerId: containerID,
		Verbose:     true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get pod sandbox info: %w", err)
	}

	podStatusResponseInfo, err := ContainerStatus(reply)
	if err != nil {
		return "", err
	}

	namespaces := podStatusResponseInfo.RunTimeSpec.Linux.Namespaces
	for _, namespace := range namespaces {
		if namespace.Type == criotypes.ContainerNetworkNamespace {
			return namespace.Path, nil
		}
	}
	return "", fmt.Errorf("could not figure out container %s netns: %v", containerID, err)
}

func ContainerStatus(containerStatus *crioruntime.ContainerStatusResponse) (criotypes.ContainerStatusResponse, error) {
	var podStatusResponseInfo criotypes.ContainerStatusResponse
	info, wasFound := containerStatus.GetInfo()["info"]
	if !wasFound {
		return criotypes.ContainerStatusResponse{}, fmt.Errorf("no container info found")
	}
	if err := json.Unmarshal([]byte(info), &podStatusResponseInfo); err != nil {
		if e, ok := err.(*json.SyntaxError); ok {
			return criotypes.ContainerStatusResponse{},
				fmt.Errorf(
					"error unmarshaling cri-o's response: syntax error at byte offset: %d. Error: %w",
					e.Offset,
					e,
				)
		}
		return criotypes.ContainerStatusResponse{}, err
	}
	return podStatusResponseInfo, nil
}

func criServerAddress(criSocketPath string) string {
	return fmt.Sprintf("unix://%s", criSocketPath)
}
