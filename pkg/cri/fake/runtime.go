package fake

import (
	"context"
	"crypto/md5" // #nosec
	"encoding/hex"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

type Runtime struct {
	cache map[string]string
}

func NewFakeRuntime(pods ...v1.Pod) *Runtime {
	runtimeCache := map[string]string{}

	for i := range pods {
		hash := md5.Sum([]byte(pods[i].GetName())) // #nosec
		runtimeCache[pods[i].GetName()] = hex.EncodeToString(hash[:])
	}
	return &Runtime{cache: runtimeCache}
}

func (r *Runtime) NetworkNamespace(_ context.Context, podName string, podNamespace string) (string, error) {
	if netnsName, wasFound := r.cache[podName]; wasFound {
		return netnsName, nil
	}
	return "", fmt.Errorf("could not find a network namespace for container: %s.%s", podName, podNamespace)
}

func (r *Runtime) PodSandboxID(_ context.Context, podName string, podNamespace string) (string, error) {
	if netnsName, wasFound := r.cache[podName]; wasFound {
		return netnsName, nil
	}
	return "", fmt.Errorf("could not find a PodSandboxID for pod: %s.%s", podName, podNamespace)
}
