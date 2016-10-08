/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

// IsValidService ...
func IsValidService(kubeClient *unversioned.Client, nsName string) (*api.Service, error) {
	if nsName == "" {
		return nil, fmt.Errorf("empty string is not a valid service name")
	}

	ns, name, err := ParseNameNS(nsName)
	if err != nil {
		return nil, err
	}
	return kubeClient.Services(ns).Get(name)
}

// ParseNameNS ...
func ParseNameNS(input string) (string, string, error) {
	nsName := strings.Split(input, "/")
	if len(nsName) != 2 {
		return "", "", fmt.Errorf("invalid format (namespace/name) found in '%v'", input)
	}

	return nsName[0], nsName[1], nil
}

// GetNodeIP returns the IP address of a node in the cluster
func GetNodeIP(kubeClient *unversioned.Client, name string) string {
	var externalIP string
	node, err := kubeClient.Nodes().Get(name)
	if err != nil {
		return externalIP
	}

	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeExternalIP {
			if address.Address != "" {
				externalIP = address.Address
				break
			}
		}

		if externalIP == "" && address.Type == api.NodeLegacyHostIP {
			externalIP = address.Address
		}
	}
	return externalIP
}

// PodInfo contains runtime information about the pod
type PodInfo struct {
	Name      string
	Namespace string
	NodeIP    string
	Labels    map[string]string
}

// GetPodDetails returns runtime information about the pod:
// name, namespace and IP of the node where it is running
func GetPodDetails(kubeClient *unversioned.Client) (*PodInfo, error) {
	podName := os.Getenv("POD_NAME")
	podNs := os.Getenv("POD_NAMESPACE")

	if podName == "" && podNs == "" {
		return nil, fmt.Errorf("unable to get POD information (missing POD_NAME or POD_NAMESPACE environment variable")
	}

	pod, _ := kubeClient.Pods(podNs).Get(podName)
	if pod == nil {
		return nil, fmt.Errorf("unable to get POD information")
	}

	return &PodInfo{
		Name:      podName,
		Namespace: podNs,
		NodeIP:    GetNodeIP(kubeClient, pod.Spec.NodeName),
		Labels:    pod.GetLabels(),
	}, nil
}
