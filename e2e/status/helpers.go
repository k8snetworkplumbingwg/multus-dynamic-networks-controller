package status

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/e2e/client"
)

type NetworkStatusPredicate func(networkStatus nettypes.NetworkStatus) bool

func FilterPodsNetworkStatus(clients *client.E2EClient, namespace, podName string, p NetworkStatusPredicate) []nettypes.NetworkStatus {
	pods, err := clients.ListPods(namespace, fmt.Sprintf("app=%s", podName))
	if err != nil {
		return nil
	}
	var podNetworkStatus []nettypes.NetworkStatus
	for _, netStatus := range PodDynamicNetworks(&pods.Items[0]) {
		if p(netStatus) {
			podNetworkStatus = append(podNetworkStatus, netStatus)
		}
	}
	return podNetworkStatus
}

func PodDynamicNetworks(pod *corev1.Pod) []nettypes.NetworkStatus {
	newNetsAnnotations, wasFound := pod.Annotations[nettypes.NetworkStatusAnnot]
	if !wasFound {
		return nil
	}
	var dynamicNetworksAnnotations []nettypes.NetworkStatus
	if err := json.Unmarshal([]byte(newNetsAnnotations), &dynamicNetworksAnnotations); err != nil {
		return nil
	}
	return dynamicNetworksAnnotations
}

func CleanMACAddressesFromStatus() func(status []nettypes.NetworkStatus) []nettypes.NetworkStatus {
	return func(status []nettypes.NetworkStatus) []nettypes.NetworkStatus {
		for i := range status {
			status[i].Mac = ""
		}
		return status
	}
}
