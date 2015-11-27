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
	"fmt"
	"math/rand"
	"testing"
	"time"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

// newLoadBalancerController create a loadbalancer controller.
func newLoadBalancerController(t *testing.T, cm *fakeClusterManager, masterUrl string) *loadBalancerController {
	client := client.NewOrDie(&client.Config{Host: masterUrl, Version: testapi.Default.Version()})
	lb, err := NewLoadBalancerController(client, cm.ClusterManager, 1*time.Second)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return lb
}

// toHTTPIngressPaths converts the given pathMap to a list of HTTPIngressPaths.
func toHTTPIngressPaths(pathMap map[string]string) []extensions.HTTPIngressPath {
	httpPaths := []extensions.HTTPIngressPath{}
	for path, backend := range pathMap {
		httpPaths = append(httpPaths, extensions.HTTPIngressPath{
			Path: path,
			Backend: extensions.IngressBackend{
				ServiceName: backend,
				ServicePort: testBackendPort,
			},
		})
	}
	return httpPaths
}

// toIngressRules converts the given ingressRule map to a list of IngressRules.
func toIngressRules(hostRules map[string]fakeIngressRuleValueMap) []extensions.IngressRule {
	rules := []extensions.IngressRule{}
	for host, pathMap := range hostRules {
		rules = append(rules, extensions.IngressRule{
			Host: host,
			IngressRuleValue: extensions.IngressRuleValue{
				HTTP: &extensions.HTTPIngressRuleValue{
					Paths: toHTTPIngressPaths(pathMap),
				},
			},
		})
	}
	return rules
}

// newIngress returns a new Ingress with the given path map.
func newIngress(hostRules map[string]fakeIngressRuleValueMap) *extensions.Ingress {
	return &extensions.Ingress{
		ObjectMeta: api.ObjectMeta{
			Name:      fmt.Sprintf("%v", util.NewUUID()),
			Namespace: api.NamespaceNone,
		},
		Spec: extensions.IngressSpec{
			Backend: &extensions.IngressBackend{
				ServiceName: defaultBackendName(testClusterName),
				ServicePort: testBackendPort,
			},
			Rules: toIngressRules(hostRules),
		},
		Status: extensions.IngressStatus{
			LoadBalancer: api.LoadBalancerStatus{
				Ingress: []api.LoadBalancerIngress{
					{IP: testIPManager.ip()},
				},
			},
		},
	}
}

// validIngress returns a valid Ingress.
func validIngress() *extensions.Ingress {
	return newIngress(map[string]fakeIngressRuleValueMap{
		"foo.bar.com": testPathMap,
	})
}

// getKey returns the key for an ingress.
func getKey(ing *extensions.Ingress, t *testing.T) string {
	key, err := keyFunc(ing)
	if err != nil {
		t.Fatalf("Unexpected error getting key for Ingress %v: %v", ing.Name, err)
	}
	return key
}

// nodePortManager is a helper to allocate ports to services and
// remember the allocations.
type nodePortManager struct {
	portMap map[string]int
	start   int
	end     int
}

// randPort generated pseudo random port numbers.
func (p *nodePortManager) getNodePort(svcName string) int {
	if port, ok := p.portMap[svcName]; ok {
		return port
	}
	p.portMap[svcName] = rand.Intn(p.end-p.start) + p.start
	return p.portMap[svcName]
}

// toNodePortSvcNames converts all service names in the given map to gce node
// port names, eg foo -> k8-be-<foo nodeport>
func (p *nodePortManager) toNodePortSvcNames(inputMap map[string]fakeIngressRuleValueMap) map[string]fakeIngressRuleValueMap {
	expectedMap := map[string]fakeIngressRuleValueMap{}
	for host, rules := range inputMap {
		ruleMap := fakeIngressRuleValueMap{}
		for path, svc := range rules {
			ruleMap[path] = beName(int64(p.portMap[svc]))
		}
		expectedMap[host] = ruleMap
	}
	return expectedMap
}

func newPortManager(st, end int) *nodePortManager {
	return &nodePortManager{map[string]int{}, st, end}
}

// addIngress adds an ingress to the loadbalancer controllers ingress store. If
// a nodePortManager is supplied, it also adds all backends to the service store
// with a nodePort acquired through it.
func addIngress(lbc *loadBalancerController, ing *extensions.Ingress, pm *nodePortManager) {
	lbc.ingLister.Store.Add(ing)
	if pm == nil {
		return
	}
	for _, rule := range ing.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			svc := &api.Service{
				ObjectMeta: api.ObjectMeta{
					Name:      path.Backend.ServiceName,
					Namespace: ing.Namespace,
				},
			}
			var svcPort api.ServicePort
			switch path.Backend.ServicePort.Kind {
			case util.IntstrInt:
				svcPort = api.ServicePort{Port: path.Backend.ServicePort.IntVal}
			default:
				svcPort = api.ServicePort{Name: path.Backend.ServicePort.StrVal}
			}
			svcPort.NodePort = pm.getNodePort(path.Backend.ServiceName)
			svc.Spec.Ports = []api.ServicePort{svcPort}
			lbc.svcLister.Store.Add(svc)
		}
	}
}

func TestLbCreateDelete(t *testing.T) {
	cm := newFakeClusterManager(testClusterName)
	lbc := newLoadBalancerController(t, cm, "")
	inputMap1 := map[string]fakeIngressRuleValueMap{
		"foo.example.com": {
			"/foo1": "foo1svc",
			"/foo2": "foo2svc",
		},
		"bar.example.com": {
			"/bar1": "bar1svc",
			"/bar2": "bar2svc",
		},
	}
	inputMap2 := map[string]fakeIngressRuleValueMap{
		"baz.foobar.com": {
			"/foo": "foo1svc",
			"/bar": "bar1svc",
		},
	}
	pm := newPortManager(1, 65536)
	ings := []*extensions.Ingress{}
	for _, m := range []map[string]fakeIngressRuleValueMap{inputMap1, inputMap2} {
		newIng := newIngress(m)
		addIngress(lbc, newIng, pm)
		ingStoreKey := getKey(newIng, t)
		lbc.sync(ingStoreKey)
		l7, err := cm.l7Pool.Get(ingStoreKey)
		if err != nil {
			t.Fatalf("%v", err)
		}
		cm.fakeLbs.checkUrlMap(t, l7, pm.toNodePortSvcNames(m))
		ings = append(ings, newIng)
	}
	lbc.ingLister.Store.Delete(ings[0])
	lbc.sync(getKey(ings[0], t))

	// BackendServices associated with ports of deleted Ingress' should get gc'd
	// when the Ingress is deleted, regardless of the service. At the same time
	// we shouldn't pull shared backends out from existing loadbalancers.
	unexpected := []int{pm.portMap["foo2svc"], pm.portMap["bar2svc"]}
	expected := []int{pm.portMap["foo1svc"], pm.portMap["bar1svc"]}

	for _, port := range expected {
		if _, err := cm.backendPool.Get(int64(port)); err != nil {
			t.Fatalf("%v", err)
		}
	}
	for _, port := range unexpected {
		if be, err := cm.backendPool.Get(int64(port)); err == nil {
			t.Fatalf("Found backend %+v for port %v", be, port)
		}
	}
	lbc.ingLister.Store.Delete(ings[1])
	lbc.sync(getKey(ings[1], t))

	// No cluster resources (except the defaults used by the cluster manager)
	// should exist at this point.
	for _, port := range expected {
		if be, err := cm.backendPool.Get(int64(port)); err == nil {
			t.Fatalf("Found backend %+v for port %v", be, port)
		}
	}
	if len(cm.fakeLbs.fw) != 0 || len(cm.fakeLbs.um) != 0 || len(cm.fakeLbs.tp) != 0 {
		t.Fatalf("Loadbalancer leaked resources")
	}
	for _, lbName := range []string{getKey(ings[0], t), getKey(ings[1], t)} {
		if l7, err := cm.l7Pool.Get(lbName); err == nil {
			t.Fatalf("Found unexpected loadbalandcer %+v: %v", l7, err)
		}
	}
}

func TestLbFaultyUpdate(t *testing.T) {
	cm := newFakeClusterManager(testClusterName)
	lbc := newLoadBalancerController(t, cm, "")
	inputMap := map[string]fakeIngressRuleValueMap{
		"foo.example.com": {
			"/foo1": "foo1svc",
			"/foo2": "foo2svc",
		},
		"bar.example.com": {
			"/bar1": "bar1svc",
			"/bar2": "bar2svc",
		},
	}
	ing := newIngress(inputMap)
	pm := newPortManager(1, 65536)
	addIngress(lbc, ing, pm)

	ingStoreKey := getKey(ing, t)
	lbc.sync(ingStoreKey)
	l7, err := cm.l7Pool.Get(ingStoreKey)
	if err != nil {
		t.Fatalf("%v", err)
	}
	cm.fakeLbs.checkUrlMap(t, l7, pm.toNodePortSvcNames(inputMap))

	// Change the urlmap directly through the lb pool, resync, and
	// make sure the controller corrects it.
	l7.UpdateUrlMap(gceUrlMap{
		"foo.example.com": {
			"/foo1": &compute.BackendService{SelfLink: "foo2svc"},
		},
	})

	lbc.sync(ingStoreKey)
	cm.fakeLbs.checkUrlMap(t, l7, pm.toNodePortSvcNames(inputMap))
}

func TestLbDefaulting(t *testing.T) {
	cm := newFakeClusterManager(testClusterName)
	lbc := newLoadBalancerController(t, cm, "")
	// Make sure the controller plugs in the default values accepted by GCE.
	ing := newIngress(map[string]fakeIngressRuleValueMap{"": {"": "foo1svc"}})
	pm := newPortManager(1, 65536)
	addIngress(lbc, ing, pm)

	ingStoreKey := getKey(ing, t)
	lbc.sync(ingStoreKey)
	l7, err := cm.l7Pool.Get(ingStoreKey)
	if err != nil {
		t.Fatalf("%v", err)
	}
	expectedMap := map[string]fakeIngressRuleValueMap{defaultHost: {defaultPath: "foo1svc"}}
	cm.fakeLbs.checkUrlMap(t, l7, pm.toNodePortSvcNames(expectedMap))
}

func TestLbNoService(t *testing.T) {
	cm := newFakeClusterManager(testClusterName)
	lbc := newLoadBalancerController(t, cm, "")
	inputMap := map[string]fakeIngressRuleValueMap{
		"foo.example.com": {
			"/foo1": "foo1svc",
		},
	}
	ing := newIngress(inputMap)
	ing.Spec.Backend.ServiceName = "foo1svc"
	ingStoreKey := getKey(ing, t)

	// Adds ingress to store, but doesn't create an associated service.
	// This will still create the associated loadbalancer, it will just
	// have empty rules. The rules will get corrected when the service
	// pops up.
	addIngress(lbc, ing, nil)
	lbc.sync(ingStoreKey)

	l7, err := cm.l7Pool.Get(ingStoreKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Creates the service, next sync should have complete url map.
	pm := newPortManager(1, 65536)
	addIngress(lbc, ing, pm)
	lbc.enqueueIngressForService(&api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo1svc",
			Namespace: ing.Namespace,
		},
	})
	// TODO: This will hang if the previous step failed to insert into queue
	key, _ := lbc.ingQueue.queue.Get()
	lbc.sync(key.(string))

	inputMap[defaultBackendKey] = map[string]string{
		defaultBackendKey: "foo1svc",
	}
	expectedMap := pm.toNodePortSvcNames(inputMap)
	cm.fakeLbs.checkUrlMap(t, l7, expectedMap)
}

// TODO: Test lb status update when annotation stabilize
