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
)

func newLoadBalancerPool(f LoadBalancers, t *testing.T) LoadBalancerPool {
	cm := newFakeClusterManager("test")
	return NewLoadBalancerPool(f, cm.backendPool, testDefaultBeNodePort)
}

func TestCreateLoadBalancer(t *testing.T) {
	lbName := "test"
	f := newFakeLoadBalancers(lbName)
	pool := newLoadBalancerPool(f, t)
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
	um1 := gceUrlMap{
		"bar.example.com": {
			"/bar2": &compute.BackendService{SelfLink: "bar2svc"},
		},
	}
	um2 := gceUrlMap{
		"foo.example.com": {
			"/foo1": &compute.BackendService{SelfLink: "foo1svc"},
			"/foo2": &compute.BackendService{SelfLink: "foo2svc"},
		},
		"bar.example.com": {
			"/bar1": &compute.BackendService{SelfLink: "bar1svc"},
		},
	}
	um2.putDefaultBackend(&compute.BackendService{SelfLink: "default"})

	lbName := "test"
	f := newFakeLoadBalancers(lbName)
	pool := newLoadBalancerPool(f, t)
	pool.Add(lbName)
	l7, err := pool.Get(lbName)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, ir := range []gceUrlMap{um1, um2} {
		if err := l7.UpdateUrlMap(ir); err != nil {
			t.Fatalf("%v", err)
		}
	}
	// The final map doesn't contain /bar2
	expectedMap := map[string]fakeIngressRuleValueMap{
		defaultBackendKey: {
			defaultBackendKey: "default",
		},
		"foo.example.com": {
			"/foo1": "foo1svc",
			"/foo2": "foo2svc",
		},
		"bar.example.com": {
			"/bar1": "bar1svc",
		},
	}
	f.checkUrlMap(t, l7, expectedMap)
}
