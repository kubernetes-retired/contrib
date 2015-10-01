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

package main

import (
	"testing"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/util/sets"
)

func newNodePool(f InstanceGroups, defaultIgName string, t *testing.T) NodePool {
	pool, err := NewNodePool(f, defaultIgName)
	if err != nil || pool == nil {
		t.Fatalf("%v", err)
	}
	return pool
}

func TestNewNodePoolCreate(t *testing.T) {
	f := newFakeInstanceGroups(sets.NewString())
	defaultIgName := defaultInstanceGroupName(testClusterName)
	newNodePool(f, defaultIgName, t)

	// Test that creating a node pool creates a default instance group
	// after checking that it doesn't already exist.
	if f.instanceGroup != defaultIgName {
		t.Fatalf("Default instance group not created, got %v expected %v.",
			f.instanceGroup, defaultIgName)
	}

	if f.calls[0] != Get {
		t.Fatalf("Default instance group was created without existence check.")
	}

	f.getResult = &compute.InstanceGroup{}
	pool := newNodePool(f, "newDefaultIgName", t)
	for _, call := range f.calls {
		if call == Create {
			t.Fatalf("Tried to create instance group when one already exists.")
		}
	}
	if pool.(*Instances).defaultIg != f.getResult {
		t.Fatalf("Default instance group not created, got %v expected %v.",
			f.instanceGroup, defaultIgName)
	}
}

func TestNodePoolSync(t *testing.T) {
	f := newFakeInstanceGroups(sets.NewString(
		[]string{"n1", "n2"}...))
	defaultIgName := defaultInstanceGroupName(testClusterName)
	pool := newNodePool(f, defaultIgName, t)

	// KubeNodes: n1
	// GCENodes: n1, n2
	// Remove n2 from the instance group.

	f.calls = []int{}
	kubeNodes := sets.NewString([]string{"n1"}...)
	pool.Sync(kubeNodes.List())
	if len(f.calls) != 1 || f.calls[0] != RemoveInstances ||
		f.instances.Len() != kubeNodes.Len() ||
		!kubeNodes.IsSuperset(f.instances) {
		t.Fatalf(
			"Expected %v with instances %v, got %v with instances %+v",
			RemoveInstances, kubeNodes, f.calls, f.instances)
	}

	// KubeNodes: n1, n2
	// GCENodes: n1
	// Try to add n2 to the instance group.

	f = newFakeInstanceGroups(sets.NewString([]string{"n1"}...))
	pool = newNodePool(f, defaultIgName, t)

	f.calls = []int{}
	kubeNodes = sets.NewString([]string{"n1", "n2"}...)
	pool.Sync(kubeNodes.List())
	if len(f.calls) != 1 || f.calls[0] != AddInstances ||
		f.instances.Len() != kubeNodes.Len() ||
		!kubeNodes.IsSuperset(f.instances) {
		t.Fatalf(
			"Expected %v with instances %v, got %v with instances %+v",
			RemoveInstances, kubeNodes, f.calls, f.instances)
	}

	// KubeNodes: n1, n2
	// GCENodes: n1, n2
	// Do nothing.

	f = newFakeInstanceGroups(sets.NewString([]string{"n1", "n2"}...))
	pool = newNodePool(f, defaultIgName, t)

	f.calls = []int{}
	kubeNodes = sets.NewString([]string{"n1", "n2"}...)
	pool.Sync(kubeNodes.List())
	if len(f.calls) != 0 {
		t.Fatalf(
			"Did not expect any calls, got %+v", f.calls)
	}
}

func TestNodePoolShutdown(t *testing.T) {
	f := newFakeInstanceGroups(sets.NewString())
	f.getResult = nil
	defaultIgName := defaultInstanceGroupName(testClusterName)
	pool := newNodePool(f, defaultIgName, t)

	// Make sure the default instance group is only deleted when the pool
	// is empty.
	f.listResult = getInstanceList(sets.NewString("foo"))
	pool.Shutdown()
	if f.instanceGroup != "" {
		t.Fatalf("Did not expect an instance group, found %v", defaultIgName)
	}
}
