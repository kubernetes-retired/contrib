/*
Copyright 2017 The Kubernetes Authors.

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

package priority

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/contrib/vertical-pod-autoscaler/updater/apimock"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"math"
	"sort"
)

const (
	// ignore change smaller than 10%
	defaultUpdateThreshod = 0.10
)

// UpdatePriorityCalculator is responsible for prioritizing updates on pods.
// Can return sorted list of pods in order of update priority.
// Update priority is proportional to fraction by which resources should be increased / decreased.
// i.e. pod with 10M current memory and recommendation 20M will have higher update priority
// than pod with pod with 100M current memory and and 150M recommendation (100% increase vs 50% increase)
type UpdatePriorityCalculator struct {
	resourcesPolicy *apimock.ResourcesPolicy
	cpuPolicy       *apimock.Policy
	pods            []podPriority
	config          *UpdateConfig
}

// UpdateConfig holds configuration for UpdatePriorityCalculator
type UpdateConfig struct {
	// Minimum change of resources recommended vs actual that will trigger update
	// TODO: should have separate for Mem and CPU?
	MinChange float64
}

// NewUpdatePriorityCalculator creates new UpdatePriorityCalculator for given sesources policy and configuration.
// Policy nil means no policy restriction on update.
// If configuration is nil, default values are used.
func NewUpdatePriorityCalculator(policy *apimock.ResourcesPolicy, config *UpdateConfig) UpdatePriorityCalculator {
	if config == nil {
		config = &UpdateConfig{MinChange: defaultUpdateThreshod}
	}
	return UpdatePriorityCalculator{resourcesPolicy: policy, config: config}
}

// AddPod adds pod to priority list
func (calc *UpdatePriorityCalculator) AddPod(pod *apiv1.Pod, recommendation *apimock.Recommendation) {
	updatePriority := calc.getUpdatePriority(pod, recommendation)
	if updatePriority >= calc.config.MinChange {
		glog.V(1).Infof("Pod accepted for update %v with priority %v", pod.Name, updatePriority)
		calc.pods = append(calc.pods, podPriority{pod, updatePriority})

	} else {
		glog.V(1).Infof("Pod not accepted for update %v, priority too low: %v", pod.Name, updatePriority)
	}
}

// GetSortedPods returns list of pods ordered by priority (highest update priority first)
func (calc *UpdatePriorityCalculator) GetSortedPods() []*apiv1.Pod {
	sort.Sort(byPriority(calc.pods))

	result := make([]*apiv1.Pod, len(calc.pods))
	for i, podPrio := range calc.pods {
		result[i] = podPrio.pod
	}

	return result
}

func (calc *UpdatePriorityCalculator) getUpdatePriority(pod *apiv1.Pod, recommendation *apimock.Recommendation) float64 {
	var priority float64

	for _, podContainer := range pod.Spec.Containers {

		containerRecommendation := getContainerRecommendation(podContainer.Name, recommendation)
		if containerRecommendation == nil {
			glog.V(1).Infof("No recommendation for container %v in pod %v", podContainer.Name, pod.Name)
			continue
		}

		memoryPolicy, cpuPolicy := getContainerPolicy(podContainer.Name, calc.resourcesPolicy)

		memoryDiff := getPercentageDiff(
			podContainer.Resources.Requests.Memory(), memoryPolicy, &containerRecommendation.Memory)

		cpuDiff := getPercentageDiff(
			podContainer.Resources.Requests.Cpu(), cpuPolicy, &containerRecommendation.Cpu)

		priority += math.Abs(memoryDiff) + math.Abs(cpuDiff)
	}
	// test priority threshold
	return priority
}

func getPercentageDiff(actual *resource.Quantity, policy *apimock.Policy, recommendation *resource.Quantity) float64 {
	if recommendation == nil || recommendation.IsZero() {
		return 0
	}
	recommended := recommendation.Value()
	if policy != nil {
		if !policy.Min.IsZero() && recommendation.Value() < policy.Min.Value() {
			glog.Warningf("Recommendation outside of policy bounds : min value : %v recommended : %v",
				policy.Min.Value(), recommended)
			recommended = policy.Min.Value()
		}
		if !policy.Max.IsZero() && recommendation.Value() > policy.Max.Value() {
			glog.Warningf("Recommendation outside of policy bounds : max value : %v recommended : %v",
				policy.Max.Value(), recommended)
			recommended = policy.Max.Value()
		}
	}
	diff := recommended - actual.Value()
	return float64(diff) / float64(actual.Value())
}

func getContainerPolicy(containerName string, policy *apimock.ResourcesPolicy) (memory, cpu *apimock.Policy) {
	if policy != nil {
		for _, container := range policy.Containers {
			if containerName == container.Name {
				return &container.MemoryPolicy, &container.CpuPolicy
			}
		}
	}
	return nil, nil
}

func getContainerRecommendation(containerName string, recommendation *apimock.Recommendation) *apimock.ContainerRecommendation {
	for _, container := range recommendation.Containers {
		if containerName == container.Name {
			return &container
		}
	}
	return nil
}

type podPriority struct {
	pod      *apiv1.Pod
	priority float64
}
type byPriority []podPriority

func (list byPriority) Len() int {
	return len(list)
}
func (list byPriority) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
func (list byPriority) Less(i, j int) bool {
	// reverse ordering, highest priority first
	return list[i].priority > list[j].priority
}
