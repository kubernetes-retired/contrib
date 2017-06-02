/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package utils

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"
)

type diff struct {
	key  string
	a, b string
}

type orderedDiffs []diff

var empty struct{}

func (d orderedDiffs) Len() int      { return len(d) }
func (d orderedDiffs) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d orderedDiffs) Less(i, j int) bool {
	a, b := d[i].key, d[j].key
	if a < b {
		return true
	}
	return false
}

// GetConfigMapGroups gets all the groups in the keys
// The loadbalancer Configmap has the form of
// k -> groupName.configKey
// v -> configValue
func GetConfigMapGroups(cm map[string]string) sets.String {
	keys := MapKeys(cm)
	configMapGroups := sets.NewString()
	for _, k := range keys {
		configMapGroups.Insert(getGroupName(k))
	}
	return configMapGroups
}

// DeleteConfigMapGroups deletes all configmap groups defined in deleteCms map
func DeleteConfigMapGroups(cm map[string]string, deleteCms map[string]struct{}) map[string]string {
	for k := range cm {
		if _, ok := deleteCms[getGroupName(k)]; ok {
			delete(cm, k)
		}
	}
	return cm
}

func getGroupName(key string) string {
	return strings.Split(key, ".")[0]
}

// GetUpdatedConfigMapGroups returns a set of the diff between two configmaps.
func GetUpdatedConfigMapGroups(m1, m2 map[string]string) sets.String {
	updatedConfigMapGroups := sets.NewString()
	diff := getConfigMapDiff(m1, m2)
	for _, d := range diff {
		updatedConfigMapGroups.Insert(getGroupName(d.key))
	}
	return updatedConfigMapGroups
}

func getConfigMapDiff(oldCM, newCM map[string]string) []diff {
	if reflect.DeepEqual(oldCM, newCM) {
		return nil
	}
	oldKeys := make(map[string]string)
	for _, key := range MapKeys(oldCM) {
		oldKeys[key] = oldCM[key]
	}
	var missing []diff
	for _, key := range MapKeys(newCM) {
		if _, ok := oldKeys[key]; ok {
			delete(oldKeys, key)
			if oldCM[key] == newCM[key] {
				continue
			}
			missing = append(missing, diff{key: key, a: oldCM[key], b: newCM[key]})
			continue
		}
		missing = append(missing, diff{key: key, a: "", b: newCM[key]})
	}
	for key, value := range oldKeys {
		missing = append(missing, diff{key: key, a: value, b: ""})
	}
	sort.Sort(orderedDiffs(missing))
	return missing

}

// MapKeys return keys of a map
func MapKeys(m map[string]string) []string {
	keys := make([]string, len(m))

	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}

// GetNodeHostIP returns the provided node's IP, based on the priority:
// 1. NodeExternalIP
// 2. NodeLegacyHostIP
// 3. NodeInternalIP
func GetNodeHostIP(node api.Node) (*string, error) {
	addresses := node.Status.Addresses
	addressMap := make(map[api.NodeAddressType][]api.NodeAddress)
	for i := range addresses {
		addressMap[addresses[i].Type] = append(addressMap[addresses[i].Type], addresses[i])
	}
	if addresses, ok := addressMap[api.NodeExternalIP]; ok {
		return &addresses[0].Address, nil
	}
	if addresses, ok := addressMap[api.NodeLegacyHostIP]; ok {
		return &addresses[0].Address, nil
	}
	if addresses, ok := addressMap[api.NodeInternalIP]; ok {
		return &addresses[0].Address, nil
	}
	return nil, fmt.Errorf("Host IP unknown; known addresses: %v", addresses)
}

// Filter uses the input function f to filter the given node list, and return the filtered nodes
func Filter(nodeList *api.NodeList, f func(api.Node) bool) []api.Node {
	nodes := make([]api.Node, 0)
	for _, n := range nodeList.Items {
		if f(n) {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// NodeReady returns true if the given node is READY
func NodeReady(node api.Node) bool {
	for ix := range node.Status.Conditions {
		condition := &node.Status.Conditions[ix]
		if condition.Type == api.NodeReady {
			return condition.Status == api.ConditionTrue
		}
	}
	return false
}

// GetPoolNodePortMap fetches all the configmaps and returns a map of loadbalancer pool to node port
func GetPoolNodePortMap(client *unversioned.Client, configMapNamespace string, configMapLabelKey, configMapLabelValue string) map[string]int {
	configMapNodePortMap := make(map[string]int)
	labelSelector := labels.Set{configMapLabelKey: configMapLabelValue}.AsSelector()
	opt := api.ListOptions{LabelSelector: labelSelector}
	configmaps, err := client.ConfigMaps(configMapNamespace).List(opt)
	if err != nil {
		glog.Errorf("Error while getting the configmap list %v", err)
		return configMapNodePortMap
	}
	for _, cm := range configmaps.Items {
		cmData := cm.Data
		namespace := cmData["namespace"]
		serviceName := cmData["target-service-name"]
		name := namespace + "-" + cm.Name
		serviceObj, err := client.Services(namespace).Get(serviceName)
		if err != nil {
			glog.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
			continue
		}

		if serviceObj.Spec.Type != api.ServiceTypeNodePort {
			glog.Errorf("Service %v does not have a type nodeport", serviceName)
			continue
		}

		if len(serviceObj.Spec.Ports) == 0 {
			glog.Errorf("Could not find any port from service %v.", serviceName)
			continue
		}

		for _, port := range serviceObj.Spec.Ports {
			servicePortName := port.Name
			poolName := GetResourceName("pool", name, servicePortName)
			nodePort := int(port.NodePort)
			configMapNodePortMap[poolName] = nodePort
		}
	}
	return configMapNodePortMap
}

// GetResourceName returns given args seperated by hypen
func GetResourceName(resourceType string, names ...string) string {
	return strings.Join(names, "-") + "-" + resourceType
}

// GetUserConfigMaps gets list of all user configmaps
func GetUserConfigMaps(kubeClient *unversioned.Client, configLabelKey, configLabelValue, namespace string) map[string]struct{} {
	var opts api.ListOptions
	opts.LabelSelector = labels.Set{configLabelKey: configLabelValue}.AsSelector()
	cms, err := kubeClient.ConfigMaps(namespace).List(opts)
	if err != nil {
		glog.Infof("Error getting user configmap list %v", err)
		return nil
	}

	cmList := cms.Items
	userCms := make(map[string]struct{})
	for _, cm := range cmList {
		name := cm.Namespace + "-" + cm.Name
		userCms[name] = empty
	}

	return userCms
}
