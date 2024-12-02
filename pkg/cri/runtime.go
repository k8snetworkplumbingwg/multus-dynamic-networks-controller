package cri

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cri "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubelet/pkg/types"
)

// Runtime represents a connection to the CRI-O runtime
type Runtime struct {
	Client cri.RuntimeServiceClient
}

// NewRuntime returns a connection to the CRI runtime
func NewRuntime(socketPath string, timeout time.Duration) (*Runtime, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("path to CRI socket missing")
	}

	clientConnection, err := connect(socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("error establishing connection to CRI: %w", err)
	}

	return &Runtime{
		Client: cri.NewRuntimeServiceClient(clientConnection),
	}, nil
}

func (r *Runtime) NetworkNamespace(ctx context.Context, podUID string) (string, error) {
	podSandboxID, err := r.PodSandboxID(ctx, podUID)
	if err != nil {
		return "", err
	}

	podSandboxStatus, err := r.Client.PodSandboxStatus(ctx, &cri.PodSandboxStatusRequest{
		PodSandboxId: podSandboxID,
		Verbose:      true,
	})
	if err != nil || podSandboxStatus == nil {
		return "", fmt.Errorf("failed to PodSandboxStatus for PodSandboxId %s: %w", podSandboxID, err)
	}

	sandboxInfo := &PodSandboxStatusInfo{}

	err = json.Unmarshal([]byte(podSandboxStatus.Info[InfoKey]), sandboxInfo)
	if err != nil {
		return "", fmt.Errorf("failed to Unmarshal podSandboxStatus.Info['%s']: %w", InfoKey, err)
	}

	networkNamespace := ""

	for _, namespace := range sandboxInfo.RuntimeSpec.Linux.Namespaces {
		if namespace.Type != specs.NetworkNamespace {
			continue
		}

		networkNamespace = namespace.Path
		break
	}

	if networkNamespace == "" {
		return "", fmt.Errorf("failed to find network namespace for PodSandboxId %s: %w", podSandboxID, err)
	}

	return networkNamespace, nil
}

func (r *Runtime) PodSandboxID(ctx context.Context, podUID string) (string, error) {
	// Labels used by Kubernetes: https://github.com/kubernetes/kubernetes/blob/v1.29.2/staging/src/k8s.io/kubelet/pkg/types/labels.go#L19
	ListPodSandboxRequest, err := r.Client.ListPodSandbox(ctx, &cri.ListPodSandboxRequest{
		Filter: &cri.PodSandboxFilter{
			LabelSelector: map[string]string{
				types.KubernetesPodUIDLabel: podUID,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to ListPodSandbox for pod %s: %w", podUID, err)
	}

	if ListPodSandboxRequest == nil || ListPodSandboxRequest.Items == nil || len(ListPodSandboxRequest.Items) == 0 {
		return "", fmt.Errorf("ListPodSandbox returned 0 item for pod %s", podUID)
	}

	if len(ListPodSandboxRequest.Items) > 1 {
		return "", fmt.Errorf("ListPodSandbox returned more than 1 item for pod %s", podUID)
	}

	return ListPodSandboxRequest.Items[0].Id, nil
}

func connect(socketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("endpoint is not set")
	}

	conn, err := grpc.NewClient(
		criServerAddress(socketPath),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: timeout}),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to endpoint '%s': %v", socketPath, err)
	}
	return conn, nil
}

func criServerAddress(criSocketPath string) string {
	return fmt.Sprintf("unix://%s", criSocketPath)
}
