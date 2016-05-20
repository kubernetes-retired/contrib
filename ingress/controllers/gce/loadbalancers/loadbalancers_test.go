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
	"k8s.io/contrib/ingress/controllers/gce/backends"
	"k8s.io/contrib/ingress/controllers/gce/healthchecks"
	"k8s.io/contrib/ingress/controllers/gce/instances"
	"k8s.io/contrib/ingress/controllers/gce/utils"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	testDefaultBeNodePort = int64(3000)
	defaultZone           = "default-zone"
)

func newFakeLoadBalancerPool(f LoadBalancers, t *testing.T) LoadBalancerPool {
	fakeBackends := backends.NewFakeBackendServices()
	fakeIGs := instances.NewFakeInstanceGroups(sets.NewString())
	fakeHCs := healthchecks.NewFakeHealthChecks()
	namer := utils.Namer{}
	healthChecker := healthchecks.NewHealthChecker(fakeHCs, "/", namer)
	backendPool := backends.NewBackendPool(
		fakeBackends, healthChecker, instances.NewNodePool(fakeIGs, defaultZone), namer, []int64{}, false)
	return NewLoadBalancerPool(f, backendPool, testDefaultBeNodePort, namer)
}

func TestCreateHTTPLoadBalancer(t *testing.T) {
	// This should NOT create the forwarding rule and target proxy
	// associated with the HTTPS branch of this loadbalancer.
	lbInfo := &L7RuntimeInfo{Name: "test", AllowHTTP: true}
	f := NewFakeLoadBalancers(lbInfo.Name)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbInfo)
	l7, err := pool.Get(lbInfo.Name)
	if err != nil || l7 == nil {
		t.Fatalf("Expected l7 not created")
	}
	um, err := f.GetUrlMap(f.umName())
	if err != nil ||
		um.DefaultService != pool.(*L7s).glbcDefaultBackend.SelfLink {
		t.Fatalf("%v", err)
	}
	tp, err := f.GetTargetHttpProxy(f.tpName(false))
	if err != nil || tp.UrlMap != um.SelfLink {
		t.Fatalf("%v", err)
	}
	fw, err := f.GetGlobalForwardingRule(f.fwName(false))
	if err != nil || fw.Target != tp.SelfLink {
		t.Fatalf("%v", err)
	}
}

func TestCreateHTTPSLoadBalancer(t *testing.T) {
	// This should NOT create the forwarding rule and target proxy
	// associated with the HTTP branch of this loadbalancer.
	lbInfo := &L7RuntimeInfo{
		Name:      "test",
		AllowHTTP: false,
		TLS:       &TLSCerts{Key: "key", Cert: "cert"},
	}
	f := NewFakeLoadBalancers(lbInfo.Name)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbInfo)
	l7, err := pool.Get(lbInfo.Name)
	if err != nil || l7 == nil {
		t.Fatalf("Expected l7 not created")
	}
	um, err := f.GetUrlMap(f.umName())
	if err != nil ||
		um.DefaultService != pool.(*L7s).glbcDefaultBackend.SelfLink {
		t.Fatalf("%v", err)
	}
	tps, err := f.GetTargetHttpsProxy(f.tpName(true))
	if err != nil || tps.UrlMap != um.SelfLink {
		t.Fatalf("%v", err)
	}
	fws, err := f.GetGlobalForwardingRule(f.fwName(true))
	if err != nil || fws.Target != tps.SelfLink {
		t.Fatalf("%v", err)
	}
}

func TestCreateBothLoadBalancers(t *testing.T) {
	// This should create 2 forwarding rules and target proxies
	// but they should use the same urlmap, and have the same
	// static ip.
	lbInfo := &L7RuntimeInfo{
		Name:      "test",
		AllowHTTP: true,
		TLS:       &TLSCerts{Key: "key", Cert: "cert"},
	}
	f := NewFakeLoadBalancers(lbInfo.Name)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbInfo)
	l7, err := pool.Get(lbInfo.Name)
	if err != nil || l7 == nil {
		t.Fatalf("Expected l7 not created")
	}
	um, err := f.GetUrlMap(f.umName())
	if err != nil ||
		um.DefaultService != pool.(*L7s).glbcDefaultBackend.SelfLink {
		t.Fatalf("%v", err)
	}
	tps, err := f.GetTargetHttpsProxy(f.tpName(true))
	if err != nil || tps.UrlMap != um.SelfLink {
		t.Fatalf("%v", err)
	}
	tp, err := f.GetTargetHttpProxy(f.tpName(false))
	if err != nil || tp.UrlMap != um.SelfLink {
		t.Fatalf("%v", err)
	}
	fws, err := f.GetGlobalForwardingRule(f.fwName(true))
	if err != nil || fws.Target != tps.SelfLink {
		t.Fatalf("%v", err)
	}
	fw, err := f.GetGlobalForwardingRule(f.fwName(false))
	if err != nil || fw.Target != tp.SelfLink {
		t.Fatalf("%v", err)
	}
	ip, err := f.GetGlobalStaticIP(f.fwName(false))
	if err != nil || ip.Address != fw.IPAddress || ip.Address != fws.IPAddress {
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

	lbInfo := &L7RuntimeInfo{Name: "test", AllowHTTP: true}
	f := NewFakeLoadBalancers(lbInfo.Name)
	pool := newFakeLoadBalancerPool(f, t)
	pool.Add(lbInfo)
	l7, err := pool.Get(lbInfo.Name)
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
