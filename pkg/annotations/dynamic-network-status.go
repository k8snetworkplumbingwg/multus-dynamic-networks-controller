package annotations

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadutils "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/utils"
	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"
)

func AddDynamicIfaceToStatus(currentPod *corev1.Pod, networkSelectionElement *nettypes.NetworkSelectionElement, response *multusapi.Response) (string, error) {
	currentIfaceStatus, err := podDynamicNetworkStatus(currentPod)
	if err != nil {
		return "", err
	}

	if response != nil && response.Result != nil {
		newIfaceStatus, err := nadutils.CreateNetworkStatus(
			response.Result,
			NamespacedName(networkSelectionElement.Namespace, networkSelectionElement.Name),
			false,
			nil,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create NetworkStatus from the response: %v", err)
		}

		newIfaceString, err := json.Marshal(append(currentIfaceStatus, *newIfaceStatus))
		if err != nil {
			return "", fmt.Errorf("failed to marshall the dynamic networks status after interface creation")
		}
		return string(newIfaceString), nil
	}
	return "", fmt.Errorf("got an empty response from multus: %+v", response)
}

func DeleteDynamicIfaceFromStatus(currentPod *corev1.Pod, networkSelectionElement *nettypes.NetworkSelectionElement) (string, error) {
	currentIfaceStatus, err := podDynamicNetworkStatus(currentPod)
	if err != nil {
		return "", err
	}

	netName := NamespacedName(networkSelectionElement.Namespace, networkSelectionElement.Name)
	var newIfaceStatus []nettypes.NetworkStatus
	newIfaceStatus = make([]nettypes.NetworkStatus, 0)
	for i := range currentIfaceStatus {
		if currentIfaceStatus[i].Name == netName && currentIfaceStatus[i].Interface == networkSelectionElement.InterfaceRequest {
			continue
		}
		newIfaceStatus = append(newIfaceStatus, currentIfaceStatus[i])
	}

	newIfaceString, err := json.Marshal(newIfaceStatus)
	if err != nil {
		return "", fmt.Errorf("failed to marshall the dynamic networks status after deleting interface")
	}
	return string(newIfaceString), nil
}

func podDynamicNetworkStatus(currentPod *corev1.Pod) ([]nettypes.NetworkStatus, error) {
	var currentIfaceStatus []nettypes.NetworkStatus
	if currentIfaceStatusString, wasFound := currentPod.Annotations[nettypes.NetworkStatusAnnot]; wasFound {
		if err := json.Unmarshal([]byte(currentIfaceStatusString), &currentIfaceStatus); err != nil {
			return nil, fmt.Errorf("could not unmarshall the current dynamic annotations for pod %s: %v", podNameAndNs(currentPod), err)
		}
	}
	return currentIfaceStatus, nil
}

func podNameAndNs(currentPod *corev1.Pod) string {
	return fmt.Sprintf("%s/%s", currentPod.GetNamespace(), currentPod.GetName())
}

func NamespacedName(podNamespace string, podName string) string {
	return fmt.Sprintf("%s/%s", podNamespace, podName)
}
