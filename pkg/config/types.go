package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	// DefaultDynamicNetworksControllerConfigFile is the default path of the config file
	DefaultDynamicNetworksControllerConfigFile = "/etc/cni/net.d/multus.d/daemon-config.json"
	containerdSocketPath                       = "/run/containerd/containerd.sock"
	defaultMultusSocketPath                    = "/var/run/multus-cni/multus.sock"
)

type Multus struct {
	// path to the socket through which the controller will query the CRI
	CriSocketPath string `json:"criSocketPath"`

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
		daemonNetConf.MultusSocketPath = defaultMultusSocketPath
	}

	if daemonNetConf.CriSocketPath == "" {
		daemonNetConf.CriSocketPath = containerdSocketPath
	}

	return daemonNetConf, nil
}
