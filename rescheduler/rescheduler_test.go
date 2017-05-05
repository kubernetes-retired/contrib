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

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	kube_record "k8s.io/client-go/tools/record"
	"k8s.io/contrib/cluster-autoscaler/simulator"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
)

func TestWaitForScheduled(t *testing.T) {
	pod := createTestPod("test-pod", true, 150)
	counter := 0
	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		counter++
		if counter > 2 {
			pod.Spec.NodeName = "node1"
		}
		return true, pod, nil
	})

	podsBeingProcessed := NewPodSet()
	podsBeingProcessed.Add(pod)

	assert.True(t, podsBeingProcessed.HasId("kube-system_test-pod"))
	waitForScheduled(fakeClient, podsBeingProcessed, pod)
	assert.False(t, podsBeingProcessed.HasId("kube-system_test-pod"))
}

func TestFilterCriticalPods(t *testing.T) {
	allPods := []*apiv1.Pod{}
	podsBeingProcessed := NewPodSet()
	filtered := filterCriticalPods(allPods, podsBeingProcessed)
	assert.Equal(t, 0, len(filtered))

	allPods = []*apiv1.Pod{
		createTestPod("heapster", true, 0),
		createTestPod("random", false, 0),
		createTestPod("dns", true, 0),
	}
	filtered = filterCriticalPods(allPods, podsBeingProcessed)
	assert.Equal(t, 2, len(filtered))
	assert.Equal(t, "heapster", filtered[0].Name)
	assert.Equal(t, "dns", filtered[1].Name)

	podsBeingProcessed.Add(allPods[0])
	filtered = filterCriticalPods(allPods, podsBeingProcessed)
	assert.Equal(t, 1, len(filtered))
	assert.Equal(t, "dns", filtered[0].Name)
}

func TestReleaseTaintsOnNodes(t *testing.T) {
	updatedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})

	nodes := []*apiv1.Node{
		createTestNode("node1", 1000),
		createTestNode("node2", 1000),
		createTestNode("node3", 1000),
	}
	addTaintToNode(nodes[0], "kube-system_heapster")
	addTaintToNode(nodes[1], "kube-system_dns")

	podsBeingProcessed := NewPodSet()
	podsBeingProcessed.Add(createTestPod("heapster", true, 200))

	releaseTaintsOnNodes(fakeClient, nodes, podsBeingProcessed)
	assert.Equal(t, nodes[1].Name, getStringFromChan(updatedNodes))
	assert.Equal(t, "Nothing returned", getStringFromChan(updatedNodes))
}

func TestReleaseTaintsOnNodesDeprecated(t *testing.T) {
	updatedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})

	nodes := []*apiv1.Node{
		createTestNode("node1", 1000),
		createTestNode("node2", 1000),
		createTestNode("node3", 1000),
	}
	addTaintAnnotationToNode(nodes[0], "kube-system_heapster")
	addTaintAnnotationToNode(nodes[1], "kube-system_dns")

	releaseTaintsOnNodesDeprecated(fakeClient, nodes)
	assert.Equal(t, nodes[0].Name, getStringFromChan(updatedNodes))
	assert.Equal(t, nodes[1].Name, getStringFromChan(updatedNodes))
	assert.Equal(t, "Nothing returned", getStringFromChan(updatedNodes))
}

func TestFindNodeForPod(t *testing.T) {
	predicateChecker := simulator.NewTestPredicateChecker()
	nodes := []*apiv1.Node{
		createTestNode("node1", 500),
		createTestNode("node2", 1000),
		createTestNode("node3", 2000),
	}
	pods1 := []apiv1.Pod{
		*createTestPod("p1n1", true, 100),
		*createTestPod("p2n1", false, 300),
	}
	pods2 := []apiv1.Pod{
		*createTestPod("p1n2", false, 500),
		*createTestPod("p2n2", true, 300),
	}
	pods3 := []apiv1.Pod{
		*createTestPod("p1n3", false, 500),
		*createTestPod("p2n3", false, 500),
		*createTestPod("p3n3", false, 300),
	}

	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		listAction, ok := action.(core.ListAction)
		assert.True(t, ok)
		restrictions := listAction.GetListRestrictions().Fields.String()

		podList := &apiv1.PodList{}
		switch restrictions {
		case "spec.nodeName=node1":
			podList.Items = pods1
		case "spec.nodeName=node2":
			podList.Items = pods2
		case "spec.nodeName=node3":
			podList.Items = pods3
		default:
			t.Fatalf("unexpected list restrictions: %v", restrictions)
		}
		return true, podList, nil
	})

	pod1 := createTestPod("pod1", true, 100)
	pod2 := createTestPod("pod2", true, 500)
	pod3 := createTestPod("pod3", true, 800)
	pod4 := createTestPod("pod4", true, 2200)

	node := findNodeForPod(fakeClient, predicateChecker, nodes, pod1)
	assert.Equal(t, "node1", node.Name)

	node = findNodeForPod(fakeClient, predicateChecker, nodes, pod2)
	assert.Equal(t, "node2", node.Name)

	node = findNodeForPod(fakeClient, predicateChecker, nodes, pod3)
	assert.Equal(t, "node3", node.Name)

	node = findNodeForPod(fakeClient, predicateChecker, nodes, pod4)
	assert.Nil(t, node)

}

func TestPrepareNodeForPod(t *testing.T) {
	deletedPods := make(chan string, 10)
	fakeClient := &fake.Clientset{}
	fakeRecorder := kube_record.NewFakeRecorder(10)
	predicateChecker := simulator.NewTestPredicateChecker()

	node := createTestNode("test-node", 1000)
	podsOnNode := []apiv1.Pod{
		*createTestPod("p1", true, 150),
		*createTestPod("p2", false, 150),
		*createTestPod("p3", false, 250),
		*createTestPod("p4", false, 150),
		*createTestPod("p5", true, 150),
	}
	criticalPod := createTestPod("critical-pod", true, 500)

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: podsOnNode}, nil
	})
	fakeClient.Fake.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		deleteAction := action.(core.DeleteAction)
		deletedPods <- deleteAction.GetName()
		return true, nil, nil
	})

	err := prepareNodeForPod(fakeClient, fakeRecorder, predicateChecker, node, criticalPod)
	assert.NoError(t, err)

	assert.Equal(t, podsOnNode[2].Name, getStringFromChan(deletedPods))
	assert.Equal(t, podsOnNode[3].Name, getStringFromChan(deletedPods))
	assert.Equal(t, "Nothing returned", getStringFromChan(deletedPods))
}

func createTestPod(name string, isCritical bool, cpu int64) *apiv1.Pod {
	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
			Name:      name,
			SelfLink:  fmt.Sprintf("/api/v1/namespaces/default/pods/%s", name),
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceCPU: *resource.NewMilliQuantity(cpu, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
	if isCritical {
		pod.ObjectMeta.Annotations = map[string]string{criticalPodAnnotation: ""}
	}
	return pod
}

func createTestNode(name string, cpu int64) *apiv1.Node {
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: apiv1.NodeStatus{
			Capacity: apiv1.ResourceList{
				apiv1.ResourceCPU:    *resource.NewMilliQuantity(cpu, resource.DecimalSI),
				apiv1.ResourceMemory: *resource.NewQuantity(2*1024*1024*1024, resource.DecimalSI),
				apiv1.ResourcePods:   *resource.NewQuantity(100, resource.DecimalSI),
			},
			Conditions: []apiv1.NodeCondition{
				{
					Type:   apiv1.NodeReady,
					Status: apiv1.ConditionTrue,
				},
			},
		},
	}
	node.Status.Allocatable = node.Status.Capacity
	return node
}

func addTaintToNode(node *apiv1.Node, name string) {
	node.Spec.Taints = append(node.Spec.Taints, apiv1.Taint{
		Key:    criticalAddonsOnlyTaintKey,
		Value:  name,
		Effect: apiv1.TaintEffectNoSchedule,
	})
}

func addTaintAnnotationToNode(node *apiv1.Node, name string) {
	node.Annotations = map[string]string{
		TaintsAnnotationKey: fmt.Sprintf("[{\"key\":\"CriticalAddonsOnly\", \"value\":\"%s\"}]", name),
	}
}

func getStringFromChan(c chan string) string {
	select {
	case val := <-c:
		return val
	case <-time.After(time.Second):
		return "Nothing returned"
	}
}
