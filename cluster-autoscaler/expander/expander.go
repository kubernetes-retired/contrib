package expander

import (
	"math/rand"

	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	kube_api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// ExpansionOption describes an option to expand the cluster.
type ExpansionOption struct {
	NodeGroup cloudprovider.NodeGroup
	NodeCount int
	Debug     string
	Pods      []*kube_api.Pod
}

// RandomExpansion Selects from the expansion options at random
func RandomExpansion(expansionOptions []ExpansionOption) *ExpansionOption {
	pos := rand.Int31n(int32(len(expansionOptions)))
	return &expansionOptions[pos]
}

// MostPodsExpansion Selects the expansion option that schedules the most pods
func MostPodsExpansion(expansionOptions []ExpansionOption) *ExpansionOption {
	var maxPods int
	var maxOptions []ExpansionOption

	for _, option := range expansionOptions {
		if len(option.Pods) == maxPods {
			maxOptions = append(maxOptions, option)
		}

		if len(option.Pods) > maxPods {
			maxPods = len(option.Pods)
			maxOptions = []ExpansionOption{option}
		}
	}

	if len(maxOptions) == 0 {
		return nil
	}

	return RandomExpansion(maxOptions)
}

// LeastWasteExpansion Finds the option that wastes the least amount of CPU, then the least amount of Memory, then random
func LeastWasteExpansion(expansionOptions []ExpansionOption, nodeInfo map[string]*schedulercache.NodeInfo) *ExpansionOption {
	var leastWastedCPU int64
	var leastWastedMemory int64
	var leastWastedOptions []ExpansionOption

	for _, option := range expansionOptions {
		requestedCPU, requestedMemory := resourcesForPods(option.Pods)
		node, found := nodeInfo[option.NodeGroup.Id()]
		if !found {
			glog.Errorf("No node info for: %s", option.NodeGroup.Id())
			continue
		}

		nodeCPU, nodeMemory := resourcesForNode(node.Node())
		wastedCPU := nodeCPU.MilliValue()*int64(option.NodeCount) - requestedCPU.MilliValue()
		wastedMemory := nodeMemory.MilliValue()*int64(option.NodeCount) - requestedMemory.MilliValue()

		glog.V(1).Infof("Expanding Node Group %s would waste %d CPU, %d Memory", option.NodeGroup.Id(), wastedCPU, wastedMemory)

		if wastedCPU == leastWastedCPU && wastedMemory == leastWastedMemory {
			leastWastedOptions = append(leastWastedOptions, option)
		}

		if leastWastedOptions == nil || wastedCPU < leastWastedCPU {
			leastWastedCPU, leastWastedMemory = wastedCPU, wastedMemory
			leastWastedOptions = []ExpansionOption{option}
		}

		if wastedCPU == leastWastedCPU && wastedMemory < leastWastedMemory {
			leastWastedMemory = wastedMemory
			leastWastedOptions = []ExpansionOption{option}
		}
	}

	if len(leastWastedOptions) == 0 {
		return nil
	}

	return RandomExpansion(leastWastedOptions)
}

func resourcesForPods(pods []*kube_api.Pod) (cpu resource.Quantity, memory resource.Quantity) {
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if request, ok := container.Resources.Requests[kube_api.ResourceCPU]; ok {
				cpu.Add(request)
			}
			if request, ok := container.Resources.Requests[kube_api.ResourceMemory]; ok {
				memory.Add(request)
			}
		}
	}

	return cpu, memory
}

func resourcesForNode(node *kube_api.Node) (cpu resource.Quantity, memory resource.Quantity) {
	cpu = node.Status.Capacity[kube_api.ResourceCPU]
	memory = node.Status.Capacity[kube_api.ResourceMemory]

	return cpu, memory
}
