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

func getGroupName(key string) string {
	return strings.Split(key, ".")[0]
}

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

// Get the port service based on the name. If no name is given, return the first port found
func GetServicePort(service *api.Service, portName string) (*api.ServicePort, error) {
	if len(service.Spec.Ports) == 0 {
		return nil, fmt.Errorf("Could not find any port from service %v.", service.Name)
	}

	if portName == "" {
		return &service.Spec.Ports[0], nil
	}
	for _, p := range service.Spec.Ports {
		if p.Name == portName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("Could not find matching port %v from service %v.", portName, service.Name)
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

// GetLBConfigMapNodePortMap fetches all the configmaps and returns a map of loadbalancer configmaps to node port
func GetLBConfigMapNodePortMap(client *unversioned.Client, configMapNamespace string, configMapLabelKey, configMapLabelValue string) map[string]int {
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
		serviceObj, err := client.Services(namespace).Get(serviceName)
		if err != nil {
			glog.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
			continue
		}

		targetPort, _ := cmData["target-port-name"]
		servicePort, err := GetServicePort(serviceObj, targetPort)
		if err != nil {
			glog.Errorf("Error while getting the service port %v", err)
			continue
		}

		if servicePort.NodePort == 0 {
			glog.Warningf("Service %v does not have a nodeport", serviceName)
			continue
		}

		configMapNodePortMap[namespace+"-"+cm.Name] = int(servicePort.NodePort)
	}
	return configMapNodePortMap
}
