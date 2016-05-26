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

package backends

import (
	"testing"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/contrib/ingress/controllers/gce/healthchecks"
	"k8s.io/contrib/ingress/controllers/gce/instances"
	"k8s.io/contrib/ingress/controllers/gce/storage"
	"k8s.io/contrib/ingress/controllers/gce/utils"
	"k8s.io/kubernetes/pkg/util/sets"
)

func newBackendPool(f BackendServices, fakeIGs instances.InstanceGroups, syncWithCloud bool) BackendPool {
	namer := utils.Namer{}
	return NewBackendPool(
		f,
		healthchecks.NewHealthChecker(healthchecks.NewFakeHealthChecks(), 1, "/", namer),
		instances.NewNodePool(fakeIGs, "default-zone"), namer, []int64{}, syncWithCloud)
}

func TestBackendPoolAdd(t *testing.T) {
	f := NewFakeBackendServices()
	fakeIGs := instances.NewFakeInstanceGroups(sets.NewString())
	pool := newBackendPool(f, fakeIGs, false)
	namer := utils.Namer{}

	// Add a backend for a port, then re-add the same port and
	// make sure it corrects a broken link from the backend to
	// the instance group.
	nodePort := int64(8080)
	pool.Add(nodePort)
	beName := namer.BeName(nodePort)

	// Check that the new backend has the right port
	be, err := f.GetBackendService(beName)
	if err != nil {
		t.Fatalf("Did not find expected backend %v", beName)
	}
	if be.Port != nodePort {
		t.Fatalf("Backend %v has wrong port %v, expected %v", be.Name, be.Port, nodePort)
	}
	// Check that the instance group has the new port
	var found bool
	for _, port := range fakeIGs.Ports {
		if port == nodePort {
			found = true
		}
	}
	if !found {
		t.Fatalf("Port %v not added to instance group", nodePort)
	}

	// Mess up the link between backend service and instance group.
	// This simulates a user doing foolish things through the UI.
	f.calls = []int{}
	be, err = f.GetBackendService(beName)
	be.Backends[0].Group = "test edge hop"
	f.UpdateBackendService(be)

	pool.Add(nodePort)
	for _, call := range f.calls {
		if call == utils.Create {
			t.Fatalf("Unexpected create for existing backend service")
		}
	}
	gotBackend, _ := f.GetBackendService(beName)
	gotGroup, _ := fakeIGs.GetInstanceGroup(namer.IGName(), "default-zone")
	if gotBackend.Backends[0].Group != gotGroup.SelfLink {
		t.Fatalf(
			"Broken instance group link: %v %v",
			gotBackend.Backends[0].Group,
			gotGroup.SelfLink)
	}
}

func TestBackendPoolSync(t *testing.T) {
	// Call sync on a backend pool with a list of ports, make sure the pool
	// creates/deletes required ports.
	svcNodePorts := []int64{81, 82, 83}
	f := NewFakeBackendServices()
	fakeIGs := instances.NewFakeInstanceGroups(sets.NewString())
	pool := newBackendPool(f, fakeIGs, true)
	pool.Add(81)
	pool.Add(90)
	pool.Sync(svcNodePorts)
	pool.GC(svcNodePorts)
	if _, err := pool.Get(90); err == nil {
		t.Fatalf("Did not expect to find port 90")
	}
	for _, port := range svcNodePorts {
		if _, err := pool.Get(port); err != nil {
			t.Fatalf("Expected to find port %v", port)
		}
	}

	svcNodePorts = []int64{81}
	deletedPorts := []int64{82, 83}
	pool.GC(svcNodePorts)
	for _, port := range deletedPorts {
		if _, err := pool.Get(port); err == nil {
			t.Fatalf("Pool contains %v after deletion", port)
		}
	}

	// All these backends should be ignored because they don't belong to the cluster.
	// foo - non k8s managed backend
	// k8s-be-foo - foo is not a nodeport
	// k8s--bar--foo - too many cluster delimiters
	// k8s-be-3001--uid - another cluster tagged with uid
	unrelatedBackends := sets.NewString([]string{"foo", "k8s-be-foo", "k8s--bar--foo", "k8s-be-30001--uid"}...)
	for _, name := range unrelatedBackends.List() {
		f.CreateBackendService(&compute.BackendService{Name: name})
	}

	namer := &utils.Namer{}
	// This backend should get deleted again since it is managed by this cluster.
	f.CreateBackendService(&compute.BackendService{Name: namer.BeName(deletedPorts[0])})

	// TODO: Avoid casting.
	// Repopulate the pool with a cloud list, which now includes the 82 port
	// backend. This would happen if, say, an ingress backend is removed
	// while the controller is restarting.
	pool.(*Backends).snapshotter.(*storage.CloudListingPool).ReplenishPool()

	pool.GC(svcNodePorts)

	currBackends, _ := f.ListBackendServices()
	currSet := sets.NewString()
	for _, b := range currBackends.Items {
		currSet.Insert(b.Name)
	}
	// Port 81 still exists because it's an in-use service NodePort.
	knownBe := namer.BeName(81)
	if !currSet.Has(knownBe) {
		t.Fatalf("Expected %v to exist in backend pool", knownBe)
	}
	currSet.Delete(knownBe)
	if !currSet.Equal(unrelatedBackends) {
		t.Fatalf("Some unrelated backends were deleted. Expected %+v, got %+v", unrelatedBackends, currSet)
	}
}

func TestBackendPoolShutdown(t *testing.T) {
	f := NewFakeBackendServices()
	fakeIGs := instances.NewFakeInstanceGroups(sets.NewString())
	pool := newBackendPool(f, fakeIGs, false)
	namer := utils.Namer{}

	pool.Add(80)
	pool.Shutdown()
	if _, err := f.GetBackendService(namer.BeName(80)); err == nil {
		t.Fatalf("%v", err)
	}

}
