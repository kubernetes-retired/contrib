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

package main

import (
	"fmt"
	"math/rand"

	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	"k8s.io/contrib/cluster-autoscaler/estimator"
	"k8s.io/kubernetes/pkg/api/resource"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/golang/glog"
)

// ExpansionOption describes an option to expand the cluster.
type ExpansionOption struct {
	nodeGroup cloudprovider.NodeGroup
	nodeCount int
	debug     string
	pods      []*apiv1.Pod
}

// ScaleUp tries to scale the cluster up. Return true if it found a way to increase the size,
// false if it didn't and error if an error occured. Assumes that all nodes in the cluster are
// ready and in sync with instance groups.
func ScaleUp(context AutoscalingContext, unschedulablePods []*apiv1.Pod, nodes []*apiv1.Node) (bool, error) {

	// From now on we only care about unschedulable pods that were marked after the newest
	// node became available for the scheduler.
	if len(unschedulablePods) == 0 {
		glog.V(1).Info("No unschedulable pods")
		return false, nil
	}

	for _, pod := range unschedulablePods {
		glog.V(1).Infof("Pod %s/%s is unschedulable", pod.Namespace, pod.Name)
	}

	expansionOptions := make([]ExpansionOption, 0)
	nodeInfos, err := GetNodeInfosForGroups(nodes, context.CloudProvider, context.ClientSet)
	if err != nil {
		return false, fmt.Errorf("failed to build node infos for node groups: %v", err)
	}

	podsRemainUnshedulable := make(map[*apiv1.Pod]struct{})
	for _, nodeGroup := range context.CloudProvider.NodeGroups() {

		currentSize, err := nodeGroup.TargetSize()
		if err != nil {
			glog.Errorf("Failed to get node group size: %v", err)
			continue
		}
		if currentSize >= nodeGroup.MaxSize() {
			// skip this node group.
			glog.V(4).Infof("Skipping node group %s - max size reached", nodeGroup.Id())
			continue
		}

		option := ExpansionOption{
			nodeGroup: nodeGroup,
			pods:      make([]*apiv1.Pod, 0),
		}

		nodeInfo, found := nodeInfos[nodeGroup.Id()]
		if !found {
			glog.Errorf("No node info for: %s", nodeGroup.Id())
			continue
		}

		for _, pod := range unschedulablePods {
			err = context.PredicateChecker.CheckPredicates(pod, nodeInfo)
			if err == nil {
				option.pods = append(option.pods, pod)
			} else {
				glog.V(2).Infof("Scale-up predicate failed: %v", err)
				podsRemainUnshedulable[pod] = struct{}{}
			}
		}
		if len(option.pods) > 0 {
			if context.EstimatorName == BinpackingEstimatorName {
				binpackingEstimator := estimator.NewBinpackingNodeEstimator(context.PredicateChecker)
				option.nodeCount = binpackingEstimator.Estimate(option.pods, nodeInfo)
			} else if context.EstimatorName == BasicEstimatorName {
				basicEstimator := estimator.NewBasicNodeEstimator()
				for _, pod := range option.pods {
					basicEstimator.Add(pod)
				}
				option.nodeCount, option.debug = basicEstimator.Estimate(nodeInfo.Node())
			} else {
				glog.Fatalf("Unrecognized estimator: %s", context.EstimatorName)
			}
			expansionOptions = append(expansionOptions, option)
		}
	}

	// Pick some expansion option.
	bestOption := BestExpansionOption(expansionOptions, nodeInfos, expanderName)
	if bestOption != nil && bestOption.nodeCount > 0 {
		glog.V(1).Infof("Best option to resize: %s", bestOption.nodeGroup.Id())
		if len(bestOption.debug) > 0 {
			glog.V(1).Info(bestOption.debug)
		}
		glog.V(1).Infof("Estimated %d nodes needed in %s", bestOption.nodeCount, bestOption.nodeGroup.Id())

		currentSize, err := bestOption.nodeGroup.TargetSize()
		if err != nil {
			return false, fmt.Errorf("failed to get node group size: %v", err)
		}
		newSize := currentSize + bestOption.nodeCount
		if newSize >= bestOption.nodeGroup.MaxSize() {
			glog.V(1).Infof("Capping size to MAX (%d)", bestOption.nodeGroup.MaxSize())
			newSize = bestOption.nodeGroup.MaxSize()
		}

		if context.MaxNodesTotal > 0 && len(nodes)+(newSize-currentSize) > context.MaxNodesTotal {
			glog.V(1).Infof("Capping size to max cluster total size (%d)", context.MaxNodesTotal)
			newSize = context.MaxNodesTotal - len(nodes) + currentSize
			if newSize < currentSize {
				return false, fmt.Errorf("max node total count already reached")
			}
		}

		glog.V(0).Infof("Scale-up: setting group %s size to %d", bestOption.nodeGroup.Id(), newSize)

		if err := bestOption.nodeGroup.IncreaseSize(newSize - currentSize); err != nil {
			return false, fmt.Errorf("failed to increase node group size: %v", err)
		}

		for _, pod := range bestOption.pods {
			context.Recorder.Eventf(pod, apiv1.EventTypeNormal, "TriggeredScaleUp",
				"pod triggered scale-up, group: %s, sizes (current/new): %d/%d", bestOption.nodeGroup.Id(), currentSize, newSize)
		}

		return true, nil
	}
	for pod := range podsRemainUnshedulable {
		context.Recorder.Event(pod, apiv1.EventTypeNormal, "NotTriggerScaleUp",
			"pod didn't trigger scale-up (it wouldn't fit if a new node is added)")
	}

	return false, nil
}

// BestExpansionOption picks the best cluster expansion option.
func BestExpansionOption(expansionOptions []ExpansionOption, nodeInfo map[string]*schedulercache.NodeInfo, expanderName string) *ExpansionOption {
	if len(expansionOptions) == 0 {
		return nil
	}

	if expanderName == RandomExpanderName {
		return randomExpansion(expansionOptions)
	} else if expanderName == MostPodsExpanderName {
		return mostPodsExpansion(expansionOptions)
	} else if expanderName == LeastWasteExpanderName {
		return leastWasteExpansion(expansionOptions, nodeInfo)
	}

	glog.Fatalf("Unrecognized expander: %s", expanderName)

	// Unreachable
	return nil
}

func randomExpansion(expansionOptions []ExpansionOption) *ExpansionOption {
	pos := rand.Int31n(int32(len(expansionOptions)))
	return &expansionOptions[pos]
}

func mostPodsExpansion(expansionOptions []ExpansionOption) *ExpansionOption {
	var maxPods int
	var maxOptions []ExpansionOption

	for _, option := range expansionOptions {
		if len(option.pods) == maxPods {
			maxOptions = append(maxOptions, option)
		}

		if len(option.pods) > maxPods {
			maxPods = len(option.pods)
			maxOptions = []ExpansionOption{option}
		}
	}

	if len(maxOptions) == 0 {
		return nil
	}

	return randomExpansion(maxOptions)
}

// Find the option that wastes the least amount of CPU, then the least amount of Memory, then random
func leastWasteExpansion(expansionOptions []ExpansionOption, nodeInfo map[string]*schedulercache.NodeInfo) *ExpansionOption {
	var leastWastedCPU int64
	var leastWastedMemory int64
	var leastWastedOptions []ExpansionOption

	for _, option := range expansionOptions {
		requestedCPU, requestedMemory := resourcesForPods(option.pods)
		node, found := nodeInfo[option.nodeGroup.Id()]
		if !found {
			glog.Errorf("No node info for: %s", option.nodeGroup.Id())
			continue
		}

		nodeCPU, nodeMemory := resourcesForNode(node.Node())
		wastedCPU := nodeCPU.MilliValue()*int64(option.nodeCount) - requestedCPU.MilliValue()
		wastedMemory := nodeMemory.MilliValue()*int64(option.nodeCount) - requestedMemory.MilliValue()

		if leastWastedOptions != nil && wastedCPU == leastWastedCPU && wastedMemory == leastWastedMemory {
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

	return randomExpansion(leastWastedOptions)
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
