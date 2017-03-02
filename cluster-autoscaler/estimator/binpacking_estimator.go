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

package estimator

import (
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/contrib/cluster-autoscaler/simulator"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// podInfo contains Pod and score that corresponds to how important it is to handle the pod first.
type podInfo struct {
	score float64
	pod   *apiv1.Pod
}

type byScoreDesc []*podInfo

func (a byScoreDesc) Len() int           { return len(a) }
func (a byScoreDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byScoreDesc) Less(i, j int) bool { return a[i].score > a[j].score }

// BinpackingNodeEstimator estimates the number of needed nodes to handle the given amount of pods.
type BinpackingNodeEstimator struct {
	predicateChecker *simulator.PredicateChecker
}

// NewBinpackingNodeEstimator builds a new BinpackingNodeEstimator.
func NewBinpackingNodeEstimator(predicateChecker *simulator.PredicateChecker) *BinpackingNodeEstimator {
	return &BinpackingNodeEstimator{
		predicateChecker: predicateChecker,
	}
}

// Estimate implements First Fit Decreasing bin-packing approximation algorithm.
// See https://en.wikipedia.org/wiki/Bin_packing_problem for more details.
// While it is a multi-dimensional bin packing (cpu, mem, ports) in most cases the main dimension
// will be cpu thus the estimated overprovisioning of 11/9 * optimal + 6/9 should be
// still be maintained.
// It is assumed that all pods from the given list can fit to nodeTemplate.
// Returns the number of nodes needed to accommodate all pods from the list.
func (estimator *BinpackingNodeEstimator) Estimate(pods []*apiv1.Pod, nodeTemplate *schedulercache.NodeInfo,
	comingNodes []*schedulercache.NodeInfo) int {

	podInfos := calculatePodScore(pods, nodeTemplate)
	sort.Sort(byScoreDesc(podInfos))

	// nodeWithPod function returns NodeInfo, which is a copy of nodeInfo argument with an additional pod scheduled on it.
	nodeWithPod := func(nodeInfo *schedulercache.NodeInfo, pod *apiv1.Pod) *schedulercache.NodeInfo {
		podsOnNode := nodeInfo.Pods()
		podsOnNode = append(podsOnNode, pod)
		newNodeInfo := schedulercache.NewNodeInfo(podsOnNode...)
		newNodeInfo.SetNode(nodeInfo.Node())
		return newNodeInfo
	}

	newNodes := make([]*schedulercache.NodeInfo, 0)
	for _, node := range comingNodes {
		newNodes = append(newNodes, node)
	}

	for _, podInfo := range podInfos {
		found := false
		for i, nodeInfo := range newNodes {
			if err := estimator.predicateChecker.CheckPredicates(podInfo.pod, nodeInfo); err == nil {
				found = true
				newNodes[i] = nodeWithPod(nodeInfo, podInfo.pod)
				break
			}
		}
		if !found {
			newNodes = append(newNodes, nodeWithPod(nodeTemplate, podInfo.pod))
		}
	}
	return len(newNodes) - len(comingNodes)
}

// Calculates score for all pods and returns podInfo structure.
// Score is defined as cpu_sum/node_capacity + mem_sum/node_capacity.
// Pods that have bigger requirements should be processed first, thus have higher scores.
func calculatePodScore(pods []*apiv1.Pod, nodeTemplate *schedulercache.NodeInfo) []*podInfo {
	podInfos := make([]*podInfo, 0, len(pods))

	for _, pod := range pods {
		cpuSum := resource.Quantity{}
		memorySum := resource.Quantity{}

		for _, container := range pod.Spec.Containers {
			if request, ok := container.Resources.Requests[apiv1.ResourceCPU]; ok {
				cpuSum.Add(request)
			}
			if request, ok := container.Resources.Requests[apiv1.ResourceMemory]; ok {
				memorySum.Add(request)
			}
		}
		score := float64(0)
		if cpuAllocatable, ok := nodeTemplate.Node().Status.Allocatable[apiv1.ResourceCPU]; ok && cpuAllocatable.MilliValue() > 0 {
			score += float64(cpuSum.MilliValue()) / float64(cpuAllocatable.MilliValue())
		}
		if memAllocatable, ok := nodeTemplate.Node().Status.Allocatable[apiv1.ResourceMemory]; ok && memAllocatable.Value() > 0 {
			score += float64(memorySum.Value()) / float64(memAllocatable.Value())
		}

		podInfos = append(podInfos, &podInfo{
			score: score,
			pod:   pod,
		})
	}
	return podInfos
}
