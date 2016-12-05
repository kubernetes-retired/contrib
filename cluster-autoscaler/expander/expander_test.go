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

package expander

import (
	"testing"

	"github.com/stretchr/testify/assert"
	kube_api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

func TestRandomExpander(t *testing.T) {
	eo1a := ExpansionOption{Debug: "EO1a"}

	ret := RandomExpansion([]ExpansionOption{eo1a})
	assert.Equal(t, *ret, eo1a)

	eo1b := ExpansionOption{Debug: "EO1b"}

	ret = RandomExpansion([]ExpansionOption{eo1a, eo1b})

	assert.True(t, assert.ObjectsAreEqual(*ret, eo1a) || assert.ObjectsAreEqual(*ret, eo1b))
}

func TestMostPods(t *testing.T) {
	eo0 := ExpansionOption{Debug: "EO0"}

	ret := MostPodsExpansion([]ExpansionOption{eo0})
	assert.Equal(t, *ret, eo0)

	eo1 := ExpansionOption{Debug: "EO1", Pods: []*kube_api.Pod{nil}}

	ret = MostPodsExpansion([]ExpansionOption{eo0, eo1})
	assert.Equal(t, *ret, eo1)

	eo1b := ExpansionOption{Debug: "EO1b", Pods: []*kube_api.Pod{nil}}

	ret = MostPodsExpansion([]ExpansionOption{eo0, eo1, eo1b})
	assert.NotEqual(t, *ret, eo0)

	assert.True(t, assert.ObjectsAreEqual(*ret, eo1) || assert.ObjectsAreEqual(*ret, eo1b))
}

type FakeNodeGroup struct {
	id string
}

func (f *FakeNodeGroup) MaxSize() int                       { return 2 }
func (f *FakeNodeGroup) MinSize() int                       { return 1 }
func (f *FakeNodeGroup) TargetSize() (int, error)           { return 2, nil }
func (f *FakeNodeGroup) IncreaseSize(delta int) error       { return nil }
func (f *FakeNodeGroup) DeleteNodes([]*kube_api.Node) error { return nil }
func (f *FakeNodeGroup) Id() string                         { return f.id }
func (f *FakeNodeGroup) Debug() string                      { return f.id }

func makeNodeInfo(cpu int64, memory int64, pods int64) *schedulercache.NodeInfo {
	node := &kube_api.Node{
		Status: kube_api.NodeStatus{
			Capacity: kube_api.ResourceList{
				kube_api.ResourceCPU:    *resource.NewMilliQuantity(cpu, resource.DecimalSI),
				kube_api.ResourceMemory: *resource.NewQuantity(memory, resource.DecimalSI),
				kube_api.ResourcePods:   *resource.NewQuantity(pods, resource.DecimalSI),
			},
		},
	}
	node.Status.Allocatable = node.Status.Capacity

	nodeInfo := schedulercache.NewNodeInfo()
	nodeInfo.SetNode(node)

	return nodeInfo
}

func TestLeastWaste(t *testing.T) {
	cpuPerPod := int64(500)
	memoryPerPod := int64(1000 * 1024 * 1024)

	balancedNodeInfo := makeNodeInfo(16*cpuPerPod, 16*memoryPerPod, 100)

	nodeMap := map[string]*schedulercache.NodeInfo{"balanced": balancedNodeInfo}

	balancedOption := ExpansionOption{NodeGroup: &FakeNodeGroup{"balanced"}, NodeCount: 1}

	// Test without any pods, one node info
	ret := LeastWasteExpansion([]ExpansionOption{balancedOption}, nodeMap)

	assert.Equal(t, *ret, balancedOption)

	pod := &kube_api.Pod{
		Spec: kube_api.PodSpec{
			Containers: []kube_api.Container{
				{
					Resources: kube_api.ResourceRequirements{
						Requests: kube_api.ResourceList{
							kube_api.ResourceCPU:    *resource.NewMilliQuantity(cpuPerPod, resource.DecimalSI),
							kube_api.ResourceMemory: *resource.NewQuantity(memoryPerPod, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	// Test with one pod, one node info
	balancedOption.Pods = []*kube_api.Pod{pod}

	ret = LeastWasteExpansion([]ExpansionOption{balancedOption}, nodeMap)

	assert.Equal(t, *ret, balancedOption)

	// Test with one pod, two node infos, one that has lots of RAM one that has less
	highmemNodeInfo := makeNodeInfo(16*cpuPerPod, 32*memoryPerPod, 100)
	nodeMap["highmem"] = highmemNodeInfo

	highmemOption := ExpansionOption{NodeGroup: &FakeNodeGroup{"highmem"}, NodeCount: 1, Pods: []*kube_api.Pod{pod}}

	ret = LeastWasteExpansion([]ExpansionOption{balancedOption, highmemOption}, nodeMap)

	assert.Equal(t, *ret, balancedOption)

	// Test with one pod, three node infos, one that has lots of RAM one that has less, and one that has less CPU
	lowcpuNodeInfo := makeNodeInfo(8*cpuPerPod, 16*memoryPerPod, 100)
	nodeMap["lowcpu"] = lowcpuNodeInfo

	lowcpuOption := ExpansionOption{NodeGroup: &FakeNodeGroup{"lowcpu"}, NodeCount: 1, Pods: []*kube_api.Pod{pod}}

	ret = LeastWasteExpansion([]ExpansionOption{balancedOption, highmemOption, lowcpuOption}, nodeMap)

	assert.Equal(t, *ret, lowcpuOption)
}
