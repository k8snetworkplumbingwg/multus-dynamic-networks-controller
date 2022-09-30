package multuscni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"k8s.io/klog/v2"

	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"
)

const (
	CmdAdd = "ADD"
	CmdDel = "DEL"
)

func MultusDelegateURL() string {
	const delegateEndpoint = "/delegate"
	return multusapi.GetAPIEndpoint(delegateEndpoint)
}

type Client interface {
	InvokeDelegate(req *multusapi.Request) (*multusapi.Response, error)
}

type HTTPClient struct {
	httpClient *http.Client
	serverURL  string
}

func NewClient(socketPath string) *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
		serverURL: MultusDelegateURL(),
	}
}

func (c *HTTPClient) InvokeDelegate(req *multusapi.Request) (*multusapi.Response, error) {
	httpResp, err := c.DoCNI(req)
	if err != nil {
		return nil, err
	}
	response := &multusapi.Response{}
	if len(httpResp) != 0 {
		if err = json.Unmarshal(httpResp, response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response '%s': %v", string(httpResp), err)
		}
	}
	return response, nil
}

func (c *HTTPClient) DoCNI(req *multusapi.Request) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CNI request %v: %v", req, err)
	}

	request, err := httpRequest(c.serverURL, data)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send CNI request: %v", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			klog.Errorf("failed closing the connection to the multus-server: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CNI result: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected CNI response status %v: '%s'", resp.StatusCode, string(body))
	}

	return body, nil
}

func httpRequest(serverURL string, payload []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, serverURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}
