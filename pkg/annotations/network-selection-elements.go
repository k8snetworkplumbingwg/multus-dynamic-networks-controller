// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package annotations code looted from
// https://github.com/k8snetworkplumbingwg/multus-cni/blob/549808011920e6c6f0dd4b78a75250d865e7c1c9/pkg/k8sclient/k8sclient.go
package annotations

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

func ParsePodNetworkAnnotations(podNetworks, defaultNamespace string) ([]*nadv1.NetworkSelectionElement, error) {
	var networks []*nadv1.NetworkSelectionElement

	klog.V(5).Infof("parsePodNetworkAnnotation: %s, %s", podNetworks, defaultNamespace)
	if podNetworks == "" {
		return nil, fmt.Errorf("parsePodNetworkAnnotation: pod annotation does not have \"network\" as key")
	}

	if strings.ContainsAny(podNetworks, "[{\"") {
		if err := json.Unmarshal([]byte(podNetworks), &networks); err != nil {
			return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to parse pod Network Attachment Selection Annotation JSON format: %v", err)
		}
	} else {
		// Comma-delimited list of network attachment object names
		for _, item := range strings.Split(podNetworks, ",") {
			// Remove leading and trailing whitespace.
			item = strings.TrimSpace(item)

			// Parse network name (i.e. <namespace>/<network name>@<ifname>)
			netNsName, networkName, netIfName, err := parsePodNetworkObjectName(item)
			if err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: %v", err)
			}

			networks = append(networks, &nadv1.NetworkSelectionElement{
				Name:             networkName,
				Namespace:        netNsName,
				InterfaceRequest: netIfName,
			})
		}
	}

	for _, n := range networks {
		if n.Namespace == "" {
			n.Namespace = defaultNamespace
		}
		if n.MacRequest != "" {
			// validate MAC address
			if _, err := net.ParseMAC(n.MacRequest); err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to mac: %v", err)
			}
		}
		if n.InfinibandGUIDRequest != "" {
			// validate GUID address
			if _, err := net.ParseMAC(n.InfinibandGUIDRequest); err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to validate infiniband GUID: %v", err)
			}
		}
		if n.IPRequest != nil {
			for _, ip := range n.IPRequest {
				// validate IP address
				if strings.Contains(ip, "/") {
					if _, _, err := net.ParseCIDR(ip); err != nil {
						return nil, fmt.Errorf("failed to parse CIDR %q: %v", ip, err)
					}
				} else if net.ParseIP(ip) == nil {
					return nil, fmt.Errorf("failed to parse IP address %q", ip)
				}
			}
		}
	}

	return networks, nil
}

func parsePodNetworkObjectName(podnetwork string) (string, string, string, error) {
	var netNsName string
	var netIfName string
	var networkName string

	klog.V(5).Infof("parsePodNetworkObjectName: %s", podnetwork)
	slashItems := strings.Split(podnetwork, "/")
	if len(slashItems) == 2 {
		netNsName = strings.TrimSpace(slashItems[0])
		networkName = slashItems[1]
	} else if len(slashItems) == 1 {
		networkName = slashItems[0]
	} else {
		return "", "", "", fmt.Errorf("parsePodNetworkObjectName: Invalid network object (failed at '/')")
	}

	atItems := strings.Split(networkName, "@")
	networkName = strings.TrimSpace(atItems[0])
	if len(atItems) == 2 {
		netIfName = strings.TrimSpace(atItems[1])
	} else if len(atItems) != 1 {
		return "", "", "", fmt.Errorf("parsePodNetworkObjectName: Invalid network object (failed at '@')")
	}

	// Check and see if each item matches the specification for valid attachment name.
	// "Valid attachment names must be comprised of units of the DNS-1123 label format"
	// [a-z0-9]([-a-z0-9]*[a-z0-9])?
	// And we allow at (@), and forward slash (/) (units separated by commas)
	// It must start and end alphanumerically.
	allItems := []string{netNsName, networkName, netIfName}
	expr := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	for i := range allItems {
		matched := expr.MatchString(allItems[i])
		if !matched && len([]rune(allItems[i])) > 0 {
			return "", "", "", fmt.Errorf("parsePodNetworkObjectName: Failed to parse: one or more items did not match comma-delimited format (must consist of lower case alphanumeric characters). Must start and end with an alphanumeric character), mismatch @ '%v'", allItems[i])
		}
	}

	klog.V(5).Infof("parsePodNetworkObjectName: parsed: %s, %s, %s", netNsName, networkName, netIfName)
	return netNsName, networkName, netIfName, nil
}

func IndexPodNetworkSelectionElements(pod *corev1.Pod) map[string]nadv1.NetworkSelectionElement {
	currentPodNetworkSelectionElements, err := networkSelectionElements(pod.GetAnnotations(), pod.GetNamespace())
	if err != nil {
		klog.Errorf("could not read pod's network selection elements: %v", *pod)
		return map[string]nadv1.NetworkSelectionElement{}
	}
	indexedNetworkSelectionElements := make(map[string]nadv1.NetworkSelectionElement)
	for k := range currentPodNetworkSelectionElements {
		netSelectionElement := currentPodNetworkSelectionElements[k]
		indexedNetworkSelectionElements[NetworkSelectionElementIndexKey(netSelectionElement)] = netSelectionElement
	}
	return indexedNetworkSelectionElements
}

func networkSelectionElements(podAnnotations map[string]string, podNamespace string) ([]nadv1.NetworkSelectionElement, error) {
	podNetworks, ok := podAnnotations[nadv1.NetworkAttachmentAnnot]
	if !ok || podNetworks == "" {
		return []nadv1.NetworkSelectionElement{}, nil
	}
	podNetworkSelectionElements, err := ParsePodNetworkAnnotations(podNetworks, podNamespace)
	if err != nil {
		klog.Errorf("failed to extract the network selection elements: %v", err)
		return nil, err
	}

	var currentPodNetworkSelectionElements []nadv1.NetworkSelectionElement
	for i := range podNetworkSelectionElements {
		currentPodNetworkSelectionElements = append(currentPodNetworkSelectionElements, *podNetworkSelectionElements[i])
	}
	return currentPodNetworkSelectionElements, nil
}

func NetworkSelectionElementIndexKey(netSelectionElement nadv1.NetworkSelectionElement) string {
	if netSelectionElement.InterfaceRequest != "" {
		return fmt.Sprintf(
			"%s/%s/%s",
			netSelectionElement.Namespace,
			netSelectionElement.Name,
			netSelectionElement.InterfaceRequest)
	}

	return fmt.Sprintf(
		"%s/%s",
		netSelectionElement.Namespace,
		netSelectionElement.Name)
}
