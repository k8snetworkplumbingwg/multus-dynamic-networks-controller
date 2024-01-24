package fake

import (
	"fmt"

	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/server/api"
)

type NetworkConfig struct {
	Cmd       string
	IfaceName string
	Response  *multusapi.Response
}

type Client struct {
	requestData map[string]*multusapi.Response
}

func NewFakeClient(currentStatus ...NetworkConfig) *Client {
	mockedClient := &Client{requestData: map[string]*multusapi.Response{}}
	for i := range currentStatus {
		mockedClient.requestData[keyFromCommandAndInterfaceName(currentStatus[i].Cmd, currentStatus[i].IfaceName)] = currentStatus[i].Response
	}
	return mockedClient
}

func (fc *Client) InvokeDelegate(multusRequest *multusapi.Request) (*multusapi.Response, error) {
	ifaceKey := key(multusRequest)
	serverReply, wasFound := fc.requestData[ifaceKey]
	if !wasFound {
		return nil, fmt.Errorf("not found: %s", ifaceKey)
	}
	return serverReply, nil
}

func key(req *multusapi.Request) string {
	cmd, wasFound := req.Env["CNI_COMMAND"]
	if !wasFound {
		return ""
	}
	ifName, wasFound := req.Env["CNI_IFNAME"]
	if !wasFound {
		return ""
	}
	return keyFromCommandAndInterfaceName(cmd, ifName)
}

func keyFromCommandAndInterfaceName(cmd string, ifName string) string {
	return fmt.Sprintf("%s_%s", cmd, ifName)
}
