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

package loadbalancers

import (
	"testing"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/contrib/Ingress/controllers/gce/backends"
	"k8s.io/contrib/Ingress/controllers/gce/healthchecks"
	"k8s.io/contrib/Ingress/controllers/gce/instances"
	"k8s.io/contrib/Ingress/controllers/gce/utils"
	"k8s.io/kubernetes/pkg/util/sets"
)

const testDefaultBeNodePort = int64(3000)

func newFakeLoadBalancerPool(f LoadBalancers, t *testing.T) LoadBalancerPool {
	fakeBackends := backends.NewFakeBackendServices()
	fakeIGs := instances.NewFakeInstanceGroups(sets.NewString())
	fakeHCs := healthchecks.NewFakeHealthChecks()
	namer := utils.Namer{}
	healthChecker := healthchecks.NewHealthChecker(fakeHCs, "/", namer)
	backendPool := backends.NewBackendPool(
		fakeBackends, healthChecker, instances.NewNodePool(fakeIGs), namer)
	return NewLoadBalancerPool(f, backendPool, testDefaultBeNodePort, namer)
}

func TestCreateLoadBalancer(t *testing.T) {
	lbName := "test"
	f := NewFakeLoadBalancers(lbName)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbName)
	l7, err := pool.Get(lbName)
	if err != nil || l7 == nil {
		t.Fatalf("Expected l7 not created")
	}
	um, err := f.GetUrlMap(f.umName())
	if err != nil ||
		um.DefaultService != pool.(*L7s).glbcDefaultBackend.SelfLink {
		t.Fatalf("%v", err)
	}
	tp, err := f.GetTargetHttpProxy(f.tpName())
	if err != nil || tp.UrlMap != um.SelfLink {
		t.Fatalf("%v", err)
	}
	fw, err := f.GetGlobalForwardingRule(f.fwName())
	if err != nil || fw.Target != tp.SelfLink {
		t.Fatalf("%v", err)
	}
}

func TestUpdateUrlMap(t *testing.T) {
	um1 := utils.GCEURLMap{
		"bar.example.com": {
			"/bar2": &compute.BackendService{SelfLink: "bar2svc"},
		},
	}
	um2 := utils.GCEURLMap{
		"foo.example.com": {
			"/foo1": &compute.BackendService{SelfLink: "foo1svc"},
			"/foo2": &compute.BackendService{SelfLink: "foo2svc"},
		},
		"bar.example.com": {
			"/bar1": &compute.BackendService{SelfLink: "bar1svc"},
		},
	}
	um2.PutDefaultBackend(&compute.BackendService{SelfLink: "default"})

	lbName := "test"
	f := NewFakeLoadBalancers(lbName)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbName)
	l7, err := pool.Get(lbName)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, ir := range []utils.GCEURLMap{um1, um2} {
		if err := l7.UpdateUrlMap(ir); err != nil {
			t.Fatalf("%v", err)
		}
	}
	// The final map doesn't contain /bar2
	expectedMap := map[string]utils.FakeIngressRuleValueMap{
		utils.DefaultBackendKey: {
			utils.DefaultBackendKey: "default",
		},
		"foo.example.com": {
			"/foo1": "foo1svc",
			"/foo2": "foo2svc",
		},
		"bar.example.com": {
			"/bar1": "bar1svc",
		},
	}
	f.CheckURLMap(t, l7, expectedMap)
}
