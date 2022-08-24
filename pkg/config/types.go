package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/maiqueb/multus-dynamic-networks-controller/pkg/cri"
)

const (
	// DefaultDynamicNetworksControllerConfigFile is the default path of the config file
	DefaultDynamicNetworksControllerConfigFile = "/etc/cni/net.d/multus.d/daemon-config.json"
	containerdSocketPath                       = "/run/containerd/containerd.sock"
	defaultMultusRunDir                        = "/var/run/multus-cni/"
)

type Multus struct {
	// path to the socket through which the controller will query the CRI
	CriSocketPath string `json:"criSocketPath"`

	// CRI-O or containerd
	CriType cri.RuntimeType `json:"criType"`

	// Points to the path of the unix domain socket through which the
	// client communicates with the multus server.
	MultusSocketPath string `json:"multusSocketPath"`
}

// LoadConfig loads the configuration for the multus daemon
func LoadConfig(configPath string) (*Multus, error) {
	config, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the config file's contents: %w", err)
	}

	daemonNetConf := &Multus{}
	if err := json.Unmarshal(config, daemonNetConf); err != nil {
		return nil, fmt.Errorf("failed to unmarshall the daemon configuration: %w", err)
	}

	if daemonNetConf.MultusSocketPath == "" {
		daemonNetConf.MultusSocketPath = defaultMultusRunDir
	}

	if daemonNetConf.CriSocketPath == "" {
		daemonNetConf.CriSocketPath = containerdSocketPath
	}

	if daemonNetConf.CriType == "" {
		daemonNetConf.CriType = cri.Containerd
	} else if isInvalidRuntime(daemonNetConf.CriType) {
		return nil, invalidRuntimeError(daemonNetConf.CriType)
	}

	return daemonNetConf, nil
}

func isInvalidRuntime(runtime cri.RuntimeType) bool {
	return runtime != cri.Containerd && runtime != cri.Crio
}

func invalidRuntimeError(runtime cri.RuntimeType) error {
	validRuntimes := []string{string(cri.Containerd), string(cri.Crio)}
	return fmt.Errorf(
		"invalid CRI type: %s. Allowed values are: %s",
		runtime,
		strings.Join(validRuntimes, ","))
}
