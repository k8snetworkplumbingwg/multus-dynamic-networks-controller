package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	// DefaultDynamicNetworksControllerConfigFile is the default path of the config file
	DefaultDynamicNetworksControllerConfigFile = "/etc/cni/net.d/multus.d/daemon-config.json"
	defaultMultusRunDir                        = "/var/run/multus-cni/"
)

type Multus struct {
	// path to the socket through which the controller will query the CRI
	CriSocketPath string `json:"criSocketPath"`

	// CRI-O or containerd
	CriType string `json:"criType"`

	// Points to the path of the unix domain socket through which the
	// client communicates with the multus server.
	SocketDir string `json:"socketDir"`
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

	if daemonNetConf.SocketDir == "" {
		daemonNetConf.SocketDir = defaultMultusRunDir
	}

	if daemonNetConf.CriSocketPath == "" {
		daemonNetConf.CriSocketPath = "/run/containerd/containerd.sock"
	}

	if daemonNetConf.CriType == "" {
		daemonNetConf.CriType = "containerd"
	}

	return daemonNetConf, nil
}
