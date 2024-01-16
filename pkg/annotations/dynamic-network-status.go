package annotations

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	nettypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadutils "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/utils"
)

func AddDynamicIfaceToStatus(currentPodNetworkStatus []nettypes.NetworkStatus, attachmentResults ...AttachmentResult) ([]nettypes.NetworkStatus, error) {
	for _, attachmentResult := range attachmentResults {
		response := attachmentResult.result
		networkSelectionElement := attachmentResult.attachment
		if response != nil && response.Result != nil {
			newIfaceStatus, err := nadutils.CreateNetworkStatus(
				response.Result,
				NamespacedName(networkSelectionElement.Namespace, networkSelectionElement.Name),
				false,
				nil,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create NetworkStatus from the response: %v", err)
			}
			currentPodNetworkStatus = append(currentPodNetworkStatus, *newIfaceStatus)
		}
	}
	return currentPodNetworkStatus, nil
}

func DeleteDynamicIfaceFromStatus(currentPodNetworkStatus []nettypes.NetworkStatus, networkSelectionElements ...nettypes.NetworkSelectionElement) ([]nettypes.NetworkStatus, error) {
	indexedStatus := IndexNetworkStatus(currentPodNetworkStatus)
	for _, networkSelectionElement := range networkSelectionElements {
		netStatusKey := fmt.Sprintf(
			"%s/%s",
			NamespacedName(networkSelectionElement.Namespace, networkSelectionElement.Name),
			networkSelectionElement.InterfaceRequest,
		)
		delete(indexedStatus, netStatusKey)
	}

	newIfaceStatus := make([]nettypes.NetworkStatus, 0)
	for networkStatusKey := range indexedStatus {
		newIfaceStatus = append(newIfaceStatus, indexedStatus[networkStatusKey])
	}

	return newIfaceStatus, nil
}

func PodDynamicNetworkStatus(currentPod *corev1.Pod) ([]nettypes.NetworkStatus, error) {
	var currentIfaceStatus []nettypes.NetworkStatus
	if currentIfaceStatusString, wasFound := currentPod.GetAnnotations()[nettypes.NetworkStatusAnnot]; wasFound {
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

func IndexNetworkStatus(currentPodNetworkStatus []nettypes.NetworkStatus) map[string]nettypes.NetworkStatus {
	indexedNetworkStatus := map[string]nettypes.NetworkStatus{}
	for i := range currentPodNetworkStatus {
		indexedNetworkStatus[NetworkStatusIndexKey(currentPodNetworkStatus[i])] = currentPodNetworkStatus[i]
	}
	return indexedNetworkStatus
}

func indexNetworkStatusWithIgnorePredicate(pod *corev1.Pod, p IgnoreStatusPredicate) map[string]nettypes.NetworkStatus {
	currentPodNetworkStatus, err := PodDynamicNetworkStatus(pod)
	if err != nil {
		return map[string]nettypes.NetworkStatus{}
	}
	indexedNetworkStatus := map[string]nettypes.NetworkStatus{}
	for i := range currentPodNetworkStatus {
		if !p(currentPodNetworkStatus[i]) {
			indexedNetworkStatus[NetworkStatusIndexKey(currentPodNetworkStatus[i])] = currentPodNetworkStatus[i]
		}
	}
	return indexedNetworkStatus
}

func NetworkStatusIndexKey(networkStatus nettypes.NetworkStatus) string {
	return fmt.Sprintf(
		"%s/%s",
		networkStatus.Name,
		networkStatus.Interface)
}

type IgnoreStatusPredicate func(status nettypes.NetworkStatus) bool

func IndexNetworkStatusIgnoringDefaultNetwork(pod *corev1.Pod) map[string]nettypes.NetworkStatus {
	return indexNetworkStatusWithIgnorePredicate(pod, func(status nettypes.NetworkStatus) bool {
		return status.Default
	})
}

func UpdatePodNetworkStatus(currentPod *corev1.Pod, attachmentsToUpdate []AttachmentResult) ([]nettypes.NetworkStatus, error) {
	var (
		toAdd    []AttachmentResult
		toRemove []nettypes.NetworkSelectionElement
	)

	for _, res := range attachmentsToUpdate {
		if res.IsValid() && !res.HasResult() {
			toRemove = append(toRemove, *res.attachment)
		}
		toAdd = append(toAdd, res)
	}

	currentIfaceStatus, err := PodDynamicNetworkStatus(currentPod)
	if err != nil {
		return nil, err
	}
	updatedNetworkStatus, err := AddDynamicIfaceToStatus(currentIfaceStatus, toAdd...)
	if err != nil {
		return nil, err
	}
	updatedNetworkStatus, err = DeleteDynamicIfaceFromStatus(updatedNetworkStatus, toRemove...)
	if err != nil {
		return nil, err
	}
	return updatedNetworkStatus, nil
}
