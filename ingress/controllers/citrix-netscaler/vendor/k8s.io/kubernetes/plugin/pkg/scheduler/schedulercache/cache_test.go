/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package schedulercache

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/labels"
	priorityutil "k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/priorities/util"
)

// TestAssumePodScheduled tests that after a pod is assumed, its information is aggregated
// on node level.
func TestAssumePodScheduled(t *testing.T) {
	nodeName := "node"
	testPods := []*api.Pod{
		makeBasePod(nodeName, "test", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test-1", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test-2", "200m", "1Ki", []api.ContainerPort{{HostPort: 8080}}),
		makeBasePod(nodeName, "test-nonzero", "", "", []api.ContainerPort{{HostPort: 80}}),
	}

	tests := []struct {
		pods []*api.Pod

		wNodeInfo *NodeInfo
	}{{
		pods: []*api.Pod{testPods[0]},
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{testPods[0]},
		},
	}, {
		pods: []*api.Pod{testPods[1], testPods[2]},
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 300,
				Memory:   1524,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 300,
				Memory:   1524,
			},
			pods: []*api.Pod{testPods[1], testPods[2]},
		},
	}, { // test non-zero request
		pods: []*api.Pod{testPods[3]},
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 0,
				Memory:   0,
			},
			nonzeroRequest: &Resource{
				MilliCPU: priorityutil.DefaultMilliCpuRequest,
				Memory:   priorityutil.DefaultMemoryRequest,
			},
			pods: []*api.Pod{testPods[3]},
		},
	}}

	for i, tt := range tests {
		cache := newSchedulerCache(time.Second, time.Second, nil)
		for _, pod := range tt.pods {
			if err := cache.AssumePod(pod); err != nil {
				t.Fatalf("AssumePod failed: %v", err)
			}
		}
		n := cache.nodes[nodeName]
		if !reflect.DeepEqual(n, tt.wNodeInfo) {
			t.Errorf("#%d: node info get=%s, want=%s", i, n, tt.wNodeInfo)
		}
	}
}

type testExpirePodStruct struct {
	pod         *api.Pod
	assumedTime time.Time
}

// TestExpirePod tests that assumed pods will be removed if expired.
// The removal will be reflected in node info.
func TestExpirePod(t *testing.T) {
	nodeName := "node"
	testPods := []*api.Pod{
		makeBasePod(nodeName, "test-1", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test-2", "200m", "1Ki", []api.ContainerPort{{HostPort: 8080}}),
	}
	now := time.Now()
	ttl := 10 * time.Second
	tests := []struct {
		pods        []*testExpirePodStruct
		cleanupTime time.Time

		wNodeInfo *NodeInfo
	}{{ // assumed pod would expires
		pods: []*testExpirePodStruct{
			{pod: testPods[0], assumedTime: now},
		},
		cleanupTime: now.Add(2 * ttl),
		wNodeInfo:   nil,
	}, { // first one would expire, second one would not.
		pods: []*testExpirePodStruct{
			{pod: testPods[0], assumedTime: now},
			{pod: testPods[1], assumedTime: now.Add(3 * ttl / 2)},
		},
		cleanupTime: now.Add(2 * ttl),
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			pods: []*api.Pod{testPods[1]},
		},
	}}

	for i, tt := range tests {
		cache := newSchedulerCache(ttl, time.Second, nil)

		for _, pod := range tt.pods {
			if err := cache.assumePod(pod.pod, pod.assumedTime); err != nil {
				t.Fatalf("assumePod failed: %v", err)
			}
		}
		// pods that have assumedTime + ttl < cleanupTime will get expired and removed
		cache.cleanupAssumedPods(tt.cleanupTime)
		n := cache.nodes[nodeName]
		if !reflect.DeepEqual(n, tt.wNodeInfo) {
			t.Errorf("#%d: node info get=%s, want=%s", i, n, tt.wNodeInfo)
		}
	}
}

// TestAddPodWillConfirm tests that a pod being Add()ed will be confirmed if assumed.
// The pod info should still exist after manually expiring unconfirmed pods.
func TestAddPodWillConfirm(t *testing.T) {
	nodeName := "node"
	now := time.Now()
	ttl := 10 * time.Second

	testPods := []*api.Pod{
		makeBasePod(nodeName, "test-1", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test-2", "200m", "1Ki", []api.ContainerPort{{HostPort: 8080}}),
	}
	tests := []struct {
		podsToAssume []*api.Pod
		podsToAdd    []*api.Pod

		wNodeInfo *NodeInfo
	}{{ // two pod were assumed at same time. But first one is called Add() and gets confirmed.
		podsToAssume: []*api.Pod{testPods[0], testPods[1]},
		podsToAdd:    []*api.Pod{testPods[0]},
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{testPods[0]},
		},
	}}

	for i, tt := range tests {
		cache := newSchedulerCache(ttl, time.Second, nil)
		for _, podToAssume := range tt.podsToAssume {
			if err := cache.assumePod(podToAssume, now); err != nil {
				t.Fatalf("assumePod failed: %v", err)
			}
		}
		for _, podToAdd := range tt.podsToAdd {
			if err := cache.AddPod(podToAdd); err != nil {
				t.Fatalf("AddPod failed: %v", err)
			}
		}
		cache.cleanupAssumedPods(now.Add(2 * ttl))
		// check after expiration. confirmed pods shouldn't be expired.
		n := cache.nodes[nodeName]
		if !reflect.DeepEqual(n, tt.wNodeInfo) {
			t.Errorf("#%d: node info get=%s, want=%s", i, n, tt.wNodeInfo)
		}
	}
}

// TestAddPodAfterExpiration tests that a pod being Add()ed will be added back if expired.
func TestAddPodAfterExpiration(t *testing.T) {
	nodeName := "node"
	ttl := 10 * time.Second
	basePod := makeBasePod(nodeName, "test", "100m", "500", []api.ContainerPort{{HostPort: 80}})
	tests := []struct {
		pod *api.Pod

		wNodeInfo *NodeInfo
	}{{
		pod: basePod,
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{basePod},
		},
	}}

	now := time.Now()
	for i, tt := range tests {
		cache := newSchedulerCache(ttl, time.Second, nil)
		if err := cache.assumePod(tt.pod, now); err != nil {
			t.Fatalf("assumePod failed: %v", err)
		}
		cache.cleanupAssumedPods(now.Add(2 * ttl))
		// It should be expired and removed.
		n := cache.nodes[nodeName]
		if n != nil {
			t.Errorf("#%d: expecting nil node info, but get=%v", i, n)
		}
		if err := cache.AddPod(tt.pod); err != nil {
			t.Fatalf("AddPod failed: %v", err)
		}
		// check after expiration. confirmed pods shouldn't be expired.
		n = cache.nodes[nodeName]
		if !reflect.DeepEqual(n, tt.wNodeInfo) {
			t.Errorf("#%d: node info get=%s, want=%s", i, n, tt.wNodeInfo)
		}
	}
}

// TestUpdatePod tests that a pod will be updated if added before.
func TestUpdatePod(t *testing.T) {
	nodeName := "node"
	ttl := 10 * time.Second
	testPods := []*api.Pod{
		makeBasePod(nodeName, "test", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test", "200m", "1Ki", []api.ContainerPort{{HostPort: 8080}}),
	}
	tests := []struct {
		podsToAssume []*api.Pod
		podsToAdd    []*api.Pod
		podsToUpdate []*api.Pod

		wNodeInfo []*NodeInfo
	}{{ // add a pod and then update it twice
		podsToAdd:    []*api.Pod{testPods[0]},
		podsToUpdate: []*api.Pod{testPods[0], testPods[1], testPods[0]},
		wNodeInfo: []*NodeInfo{{
			requestedResource: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			pods: []*api.Pod{testPods[1]},
		}, {
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{testPods[0]},
		}},
	}}

	for _, tt := range tests {
		cache := newSchedulerCache(ttl, time.Second, nil)
		for _, podToAdd := range tt.podsToAdd {
			if err := cache.AddPod(podToAdd); err != nil {
				t.Fatalf("AddPod failed: %v", err)
			}
		}

		for i := range tt.podsToUpdate {
			if i == 0 {
				continue
			}
			if err := cache.UpdatePod(tt.podsToUpdate[i-1], tt.podsToUpdate[i]); err != nil {
				t.Fatalf("UpdatePod failed: %v", err)
			}
			// check after expiration. confirmed pods shouldn't be expired.
			n := cache.nodes[nodeName]
			if !reflect.DeepEqual(n, tt.wNodeInfo[i-1]) {
				t.Errorf("#%d: node info get=%s, want=%s", i-1, n, tt.wNodeInfo)
			}
		}
	}
}

// TestExpireAddUpdatePod test the sequence that a pod is expired, added, then updated
func TestExpireAddUpdatePod(t *testing.T) {
	nodeName := "node"
	ttl := 10 * time.Second
	testPods := []*api.Pod{
		makeBasePod(nodeName, "test", "100m", "500", []api.ContainerPort{{HostPort: 80}}),
		makeBasePod(nodeName, "test", "200m", "1Ki", []api.ContainerPort{{HostPort: 8080}}),
	}
	tests := []struct {
		podsToAssume []*api.Pod
		podsToAdd    []*api.Pod
		podsToUpdate []*api.Pod

		wNodeInfo []*NodeInfo
	}{{ // Pod is assumed, expired, and added. Then it would be updated twice.
		podsToAssume: []*api.Pod{testPods[0]},
		podsToAdd:    []*api.Pod{testPods[0]},
		podsToUpdate: []*api.Pod{testPods[0], testPods[1], testPods[0]},
		wNodeInfo: []*NodeInfo{{
			requestedResource: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 200,
				Memory:   1024,
			},
			pods: []*api.Pod{testPods[1]},
		}, {
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{testPods[0]},
		}},
	}}

	now := time.Now()
	for _, tt := range tests {
		cache := newSchedulerCache(ttl, time.Second, nil)
		for _, podToAssume := range tt.podsToAssume {
			if err := cache.assumePod(podToAssume, now); err != nil {
				t.Fatalf("assumePod failed: %v", err)
			}
		}
		cache.cleanupAssumedPods(now.Add(2 * ttl))

		for _, podToAdd := range tt.podsToAdd {
			if err := cache.AddPod(podToAdd); err != nil {
				t.Fatalf("AddPod failed: %v", err)
			}
		}

		for i := range tt.podsToUpdate {
			if i == 0 {
				continue
			}
			if err := cache.UpdatePod(tt.podsToUpdate[i-1], tt.podsToUpdate[i]); err != nil {
				t.Fatalf("UpdatePod failed: %v", err)
			}
			// check after expiration. confirmed pods shouldn't be expired.
			n := cache.nodes[nodeName]
			if !reflect.DeepEqual(n, tt.wNodeInfo[i-1]) {
				t.Errorf("#%d: node info get=%s, want=%s", i-1, n, tt.wNodeInfo)
			}
		}
	}
}

// TestRemovePod tests after added pod is removed, its information should also be subtracted.
func TestRemovePod(t *testing.T) {
	nodeName := "node"
	basePod := makeBasePod(nodeName, "test", "100m", "500", []api.ContainerPort{{HostPort: 80}})
	tests := []struct {
		pod *api.Pod

		wNodeInfo *NodeInfo
	}{{
		pod: basePod,
		wNodeInfo: &NodeInfo{
			requestedResource: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			nonzeroRequest: &Resource{
				MilliCPU: 100,
				Memory:   500,
			},
			pods: []*api.Pod{basePod},
		},
	}}

	for i, tt := range tests {
		cache := newSchedulerCache(time.Second, time.Second, nil)
		if err := cache.AddPod(tt.pod); err != nil {
			t.Fatalf("AddPod failed: %v", err)
		}
		n := cache.nodes[nodeName]
		if !reflect.DeepEqual(n, tt.wNodeInfo) {
			t.Errorf("#%d: node info get=%s, want=%s", i, n, tt.wNodeInfo)
		}

		if err := cache.RemovePod(tt.pod); err != nil {
			t.Fatalf("RemovePod failed: %v", err)
		}

		n = cache.nodes[nodeName]
		if n != nil {
			t.Errorf("#%d: expecting pod deleted and nil node info, get=%s", i, n)
		}
	}
}

func BenchmarkGetNodeNameToInfoMap1kNodes30kPods(b *testing.B) {
	cache := setupCacheOf1kNodes30kPods(b)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.GetNodeNameToInfoMap()
	}
}

func BenchmarkList1kNodes30kPods(b *testing.B) {
	cache := setupCacheOf1kNodes30kPods(b)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cache.List(labels.Everything())
	}
}

func BenchmarkExpire100Pods(b *testing.B) {
	benchmarkExpire(b, 100)
}

func BenchmarkExpire1kPods(b *testing.B) {
	benchmarkExpire(b, 1000)
}

func BenchmarkExpire10kPods(b *testing.B) {
	benchmarkExpire(b, 10000)
}

func benchmarkExpire(b *testing.B, podNum int) {
	now := time.Now()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		cache := setupCacheWithAssumedPods(b, podNum, now)
		b.StartTimer()
		cache.cleanupAssumedPods(now.Add(2 * time.Second))
	}
}

func makeBasePod(nodeName, objName, cpu, mem string, ports []api.ContainerPort) *api.Pod {
	req := api.ResourceList{}
	if cpu != "" {
		req = api.ResourceList{
			api.ResourceCPU:    resource.MustParse(cpu),
			api.ResourceMemory: resource.MustParse(mem),
		}
	}
	return &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Namespace: "node_info_cache_test",
			Name:      objName,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{{
				Resources: api.ResourceRequirements{
					Requests: req,
				},
				Ports: ports,
			}},
			NodeName: nodeName,
		},
	}
}

func setupCacheOf1kNodes30kPods(b *testing.B) Cache {
	cache := newSchedulerCache(time.Second, time.Second, nil)
	for i := 0; i < 1000; i++ {
		nodeName := fmt.Sprintf("node-%d", i)
		for j := 0; j < 30; j++ {
			objName := fmt.Sprintf("%s-pod-%d", nodeName, j)
			pod := makeBasePod(nodeName, objName, "0", "0", nil)

			if err := cache.AddPod(pod); err != nil {
				b.Fatalf("AddPod failed: %v", err)
			}
		}
	}
	return cache
}

func setupCacheWithAssumedPods(b *testing.B, podNum int, assumedTime time.Time) *schedulerCache {
	cache := newSchedulerCache(time.Second, time.Second, nil)
	for i := 0; i < podNum; i++ {
		nodeName := fmt.Sprintf("node-%d", i/10)
		objName := fmt.Sprintf("%s-pod-%d", nodeName, i%10)
		pod := makeBasePod(nodeName, objName, "0", "0", nil)

		err := cache.assumePod(pod, assumedTime)
		if err != nil {
			b.Fatalf("assumePod failed: %v", err)
		}
	}
	return cache
}
