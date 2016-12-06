/*
Copyright 2016 The Kubernetes Authors.

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

package waste

import (
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/expander"
	"k8s.io/contrib/cluster-autoscaler/expander/random"
	kube_api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

type leastwaste struct {
	next expander.Strategy
}

// NewStrategy returns a strategy that selects the best scale up option based on which node group returns the least waste
func NewStrategy() expander.Strategy {
	return &leastwaste{random.NewStrategy()}
}

// BestOption Finds the option that wastes the least amount of CPU, then the least amount of Memory, then random
func (l *leastwaste) BestOption(expansionOptions []expander.Option, nodeInfo map[string]*schedulercache.NodeInfo) *expander.Option {
	var leastWastedCPU int64
	var leastWastedMemory int64
	var leastWastedOptions []expander.Option

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
			leastWastedOptions = []expander.Option{option}
		}

		if wastedCPU == leastWastedCPU && wastedMemory < leastWastedMemory {
			leastWastedMemory = wastedMemory
			leastWastedOptions = []expander.Option{option}
		}
	}

	if len(leastWastedOptions) == 0 {
		return nil
	}

	return l.next.BestOption(leastWastedOptions, nodeInfo)
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
