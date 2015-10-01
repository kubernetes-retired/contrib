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
	"testing"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	// Add used to record additions in a sync pool.
	Add = iota
	// Remove used to record removals from a sync pool.
	Remove
	// Sync used to record syncs of a sync pool.
	Sync
	// Get used to record Get from a sync pool.
	Get
	// Create used to recrod creations in a sync pool.
	Create
	// Update used to record updates in a sync pool.
	Update
	// Delete used to record deltions from a sync pool.
	Delete
	// AddInstances used to record a call to AddInstances.
	AddInstances
	// RemoveInstances used to record a call to RemoveInstances.
	RemoveInstances
)

var (
	testBackendPort       = util.IntOrString{Kind: util.IntstrInt, IntVal: 80}
	testDefaultBeNodePort = int64(3000)
	testClusterName       = "testcluster"
	testPathMap           = map[string]string{"/foo": defaultBackendName(testClusterName)}
	testIPManager         = testIP{}
)

type fakeIngressRuleValueMap map[string]string

// Loadbalancer fakes
type fakeLoadBalancers struct {
	fw   []*compute.ForwardingRule
	um   []*compute.UrlMap
	tp   []*compute.TargetHttpProxy
	name string
}

// TODO: There is some duplication between these functions and the name mungers in
// loadbalancer file.
func (f *fakeLoadBalancers) fwName() string {
	return fmt.Sprintf("%v-%v", forwardingRulePrefix, f.name)
}

func (f *fakeLoadBalancers) umName() string {
	return fmt.Sprintf("%v-%v", urlMapPrefix, f.name)
}

func (f *fakeLoadBalancers) tpName() string {
	return fmt.Sprintf("%v-%v", targetProxyPrefix, f.name)
}

func (f *fakeLoadBalancers) String() string {
	msg := fmt.Sprintf(
		"Loadbalancer %v,\nforwarding rules:\n", f.name)
	for _, fw := range f.fw {
		msg += fmt.Sprintf("\t%v\n", fw.Name)
	}
	msg += fmt.Sprintf("Target proxies\n")
	for _, tp := range f.tp {
		msg += fmt.Sprintf("\t%v\n", tp.Name)
	}
	msg += fmt.Sprintf("UrlMaps\n")
	for _, um := range f.um {
		msg += fmt.Sprintf("%v\n", um.Name)
		msg += fmt.Sprintf("\tHost Rules:\n")
		for _, hostRule := range um.HostRules {
			msg += fmt.Sprintf("\t\t%v\n", hostRule)
		}
		msg += fmt.Sprintf("\tPath Matcher:\n")
		for _, pathMatcher := range um.PathMatchers {
			msg += fmt.Sprintf("\t\t%v\n", pathMatcher.Name)
			for _, pathRule := range pathMatcher.PathRules {
				msg += fmt.Sprintf("\t\t\t%+v\n", pathRule)
			}
		}
	}
	return msg
}

// Forwarding Rule fakes
func (f *fakeLoadBalancers) GetGlobalForwardingRule(name string) (*compute.ForwardingRule, error) {
	for i := range f.fw {
		if f.fw[i].Name == name {
			return f.fw[i], nil
		}
	}
	return nil, fmt.Errorf("Forwarding rule %v not found", name)
}

func (f *fakeLoadBalancers) CreateGlobalForwardingRule(proxy *compute.TargetHttpProxy, name string, portRange string) (*compute.ForwardingRule, error) {

	rule := &compute.ForwardingRule{
		Name:       name,
		Target:     proxy.SelfLink,
		PortRange:  portRange,
		IPProtocol: "TCP",
		SelfLink:   f.fwName(),
		IPAddress:  fmt.Sprintf(testIPManager.ip()),
	}
	f.fw = append(f.fw, rule)
	return rule, nil
}

func (f *fakeLoadBalancers) SetProxyForGlobalForwardingRule(fw *compute.ForwardingRule, proxy *compute.TargetHttpProxy) error {
	for i := range f.fw {
		if f.fw[i].Name == fw.Name {
			f.fw[i].Target = proxy.SelfLink
		}
	}
	return nil
}

func (f *fakeLoadBalancers) DeleteGlobalForwardingRule(name string) error {
	fw := []*compute.ForwardingRule{}
	for i := range f.fw {
		if f.fw[i].Name != name {
			fw = append(fw, f.fw[i])
		}
	}
	f.fw = fw
	return nil
}

// UrlMaps fakes
func (f *fakeLoadBalancers) GetUrlMap(name string) (*compute.UrlMap, error) {
	for i := range f.um {
		if f.um[i].Name == name {
			return f.um[i], nil
		}
	}
	return nil, fmt.Errorf("Url Map %v not found", name)
}

func (f *fakeLoadBalancers) CreateUrlMap(backend *compute.BackendService, name string) (*compute.UrlMap, error) {
	urlMap := &compute.UrlMap{
		Name:           name,
		DefaultService: backend.SelfLink,
		SelfLink:       f.umName(),
	}
	f.um = append(f.um, urlMap)
	return urlMap, nil
}

func (f *fakeLoadBalancers) UpdateUrlMap(urlMap *compute.UrlMap) (*compute.UrlMap, error) {
	for i := range f.um {
		if f.um[i].Name == urlMap.Name {
			f.um[i] = urlMap
			return urlMap, nil
		}
	}
	return nil, nil
}

func (f *fakeLoadBalancers) DeleteUrlMap(name string) error {
	um := []*compute.UrlMap{}
	for i := range f.um {
		if f.um[i].Name != name {
			um = append(um, f.um[i])
		}
	}
	f.um = um
	return nil
}

// TargetProxies fakes
func (f *fakeLoadBalancers) GetTargetHttpProxy(name string) (*compute.TargetHttpProxy, error) {
	for i := range f.tp {
		if f.tp[i].Name == name {
			return f.tp[i], nil
		}
	}
	return nil, fmt.Errorf("Targetproxy %v not found", name)
}

func (f *fakeLoadBalancers) CreateTargetHttpProxy(urlMap *compute.UrlMap, name string) (*compute.TargetHttpProxy, error) {
	proxy := &compute.TargetHttpProxy{
		Name:     name,
		UrlMap:   urlMap.SelfLink,
		SelfLink: f.tpName(),
	}
	f.tp = append(f.tp, proxy)
	return proxy, nil
}

func (f *fakeLoadBalancers) DeleteTargetHttpProxy(name string) error {
	tp := []*compute.TargetHttpProxy{}
	for i := range f.tp {
		if f.tp[i].Name != name {
			tp = append(tp, f.tp[i])
		}
	}
	f.tp = tp
	return nil
}
func (f *fakeLoadBalancers) SetUrlMapForTargetHttpProxy(proxy *compute.TargetHttpProxy, urlMap *compute.UrlMap) error {
	for i := range f.tp {
		if f.tp[i].Name == proxy.Name {
			f.tp[i].UrlMap = urlMap.SelfLink
		}
	}
	return nil
}

func (f *fakeLoadBalancers) checkUrlMap(t *testing.T, l7 *L7, expectedMap map[string]fakeIngressRuleValueMap) {
	um, err := f.GetUrlMap(l7.um.Name)
	if err != nil || um == nil {
		t.Fatalf("%v", err)
	}
	// Check the default backend
	var d string
	if h, ok := expectedMap[defaultBackendKey]; ok {
		if d, ok = h[defaultBackendKey]; ok {
			delete(h, defaultBackendKey)
		}
		delete(expectedMap, defaultBackendKey)
	}
	// The urlmap should have a default backend, and each path matcher.
	if d != "" && l7.um.DefaultService != d {
		t.Fatalf("Expected default backend %v found %v",
			d, l7.um.DefaultService)
	}

	for _, matcher := range l7.um.PathMatchers {
		var hostname string
		// There's a 1:1 mapping between pathmatchers and hosts
		for _, hostRule := range l7.um.HostRules {
			if matcher.Name == hostRule.PathMatcher {
				if len(hostRule.Hosts) != 1 {
					t.Fatalf("Unexpected hosts in hostrules %+v", hostRule)
				}
				if d != "" && matcher.DefaultService != d {
					t.Fatalf("Expected default backend %v found %v",
						d, matcher.DefaultService)
				}
				hostname = hostRule.Hosts[0]
				break
			}
		}
		// These are all pathrules for a single host, found above
		for _, rule := range matcher.PathRules {
			if len(rule.Paths) != 1 {
				t.Fatalf("Unexpected rule in pathrules %+v", rule)
			}
			pathRule := rule.Paths[0]
			if hostMap, ok := expectedMap[hostname]; !ok {
				t.Fatalf("Expected map for host %v: %v", hostname, hostMap)
			} else if svc, ok := expectedMap[hostname][pathRule]; !ok {
				t.Fatalf("Expected rule %v in host map", pathRule)
			} else if svc != rule.Service {
				t.Fatalf("Expected service %v found %v", svc, rule.Service)
			}
			delete(expectedMap[hostname], pathRule)
			if len(expectedMap[hostname]) == 0 {
				delete(expectedMap, hostname)
			}
		}
	}
	if len(expectedMap) != 0 {
		t.Fatalf("Untranslated entries %+v", expectedMap)
	}
}

// newFakeLoadBalancers creates a fake cloud client. Name is the name
// inserted into the selfLink of the associated resources for testing.
// eg: forwardingRule.SelfLink == k8-fw-name.
func newFakeLoadBalancers(name string) *fakeLoadBalancers {
	return &fakeLoadBalancers{
		fw:   []*compute.ForwardingRule{},
		name: name,
	}
}

type fakeHealthChecks struct {
	hc *compute.HttpHealthCheck
}

func (f *fakeHealthChecks) CreateHttpHealthCheck(hc *compute.HttpHealthCheck) error {
	f.hc = hc
	return nil
}

func (f *fakeHealthChecks) GetHttpHealthCheck(name string) (*compute.HttpHealthCheck, error) {
	if f.hc == nil || f.hc.Name != name {
		return nil, fmt.Errorf("Health check %v not found.", name)
	}
	return f.hc, nil
}

func (f *fakeHealthChecks) DeleteHttpHealthCheck(name string) error {
	if f.hc == nil || f.hc.Name != name {
		return fmt.Errorf("Health check %v not found.", name)
	}
	f.hc = nil
	return nil
}

func newFakeHealthChecks() *fakeHealthChecks {
	return &fakeHealthChecks{hc: nil}
}

// BackendServices fakes
type fakeBackendServices struct {
	backendServices []*compute.BackendService
	calls           []int
}

func (f *fakeBackendServices) GetBackendService(name string) (*compute.BackendService, error) {
	f.calls = append(f.calls, Get)
	for i := range f.backendServices {
		if name == f.backendServices[i].Name {
			return f.backendServices[i], nil
		}
	}
	return nil, fmt.Errorf("Backend service %v not found", name)
}

func (f *fakeBackendServices) CreateBackendService(be *compute.BackendService) error {
	f.calls = append(f.calls, Create)
	be.SelfLink = be.Name
	f.backendServices = append(f.backendServices, be)
	return nil
}

func (f *fakeBackendServices) DeleteBackendService(name string) error {
	f.calls = append(f.calls, Delete)
	newBackends := []*compute.BackendService{}
	for i := range f.backendServices {
		if name != f.backendServices[i].Name {
			newBackends = append(newBackends, f.backendServices[i])
		}
	}
	f.backendServices = newBackends
	return nil
}

func (f *fakeBackendServices) UpdateBackendService(be *compute.BackendService) error {

	f.calls = append(f.calls, Update)
	for i := range f.backendServices {
		if f.backendServices[i].Name == be.Name {
			f.backendServices[i] = be
		}
	}
	return nil
}

func newFakeBackendServices() *fakeBackendServices {
	return &fakeBackendServices{
		backendServices: []*compute.BackendService{},
	}
}

// getInstanceList returns an instance list based on the given names.
// The names cannot contain a '.', the real gce api validates against this.
func getInstanceList(nodeNames sets.String) *compute.InstanceGroupsListInstances {
	instanceNames := nodeNames.List()
	computeInstances := []*compute.InstanceWithNamedPorts{}
	for _, name := range instanceNames {
		instanceLink := fmt.Sprintf(
			"https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
			"project", "zone", name)
		computeInstances = append(
			computeInstances, &compute.InstanceWithNamedPorts{
				Instance: instanceLink})
	}
	return &compute.InstanceGroupsListInstances{
		Items: computeInstances,
	}
}

// InstanceGroup fakes
type fakeInstanceGroups struct {
	instances     sets.String
	instanceGroup string
	ports         []int64
	getResult     *compute.InstanceGroup
	listResult    *compute.InstanceGroupsListInstances
	calls         []int
}

func (f *fakeInstanceGroups) GetInstanceGroup(name string) (*compute.InstanceGroup, error) {
	f.calls = append(f.calls, Get)
	return f.getResult, nil
}

func (f *fakeInstanceGroups) CreateInstanceGroup(name string) (*compute.InstanceGroup, error) {
	f.instanceGroup = name
	return &compute.InstanceGroup{}, nil
}

func (f *fakeInstanceGroups) DeleteInstanceGroup(name string) error {
	f.instanceGroup = ""
	return nil
}

func (f *fakeInstanceGroups) ListInstancesInInstanceGroup(name string, state string) (*compute.InstanceGroupsListInstances, error) {
	return f.listResult, nil
}

func (f *fakeInstanceGroups) AddInstancesToInstanceGroup(name string, instanceNames []string) error {
	f.calls = append(f.calls, AddInstances)
	f.instances.Insert(instanceNames...)
	return nil
}

func (f *fakeInstanceGroups) RemoveInstancesFromInstanceGroup(name string, instanceNames []string) error {
	f.calls = append(f.calls, RemoveInstances)
	f.instances.Delete(instanceNames...)
	return nil
}

func (f *fakeInstanceGroups) AddPortToInstanceGroup(ig *compute.InstanceGroup, port int64) (*compute.NamedPort, error) {
	f.ports = append(f.ports, port)
	return &compute.NamedPort{Name: beName(port), Port: port}, nil
}

func newFakeInstanceGroups(nodes sets.String) *fakeInstanceGroups {
	return &fakeInstanceGroups{
		instances:  nodes,
		listResult: getInstanceList(nodes),
	}
}

// ClusterManager fake
type fakeClusterManager struct {
	*ClusterManager
	fakeLbs      *fakeLoadBalancers
	fakeBackends *fakeBackendServices
	fakeIgs      *fakeInstanceGroups
}

// newFakeClusterManager creates a new fake ClusterManager.
func newFakeClusterManager(clusterName string) (*fakeClusterManager, error) {
	fakeLbs := newFakeLoadBalancers(clusterName)
	fakeBackends := newFakeBackendServices()
	fakeIgs := newFakeInstanceGroups(sets.NewString())
	fakeHcs := newFakeHealthChecks()

	defaultIgName := defaultInstanceGroupName(clusterName)
	defaultBeName := beName(testDefaultBeNodePort)

	nodePool, err := NewNodePool(fakeIgs, defaultIgName)
	if err != nil {
		return nil, err
	}

	backendPool, err := NewBackendPool(
		fakeBackends,
		testDefaultBeNodePort,
		&compute.InstanceGroup{
			SelfLink: defaultIgName,
		},
		&compute.HttpHealthCheck{}, fakeIgs)
	if err != nil {
		return nil, err
	}
	l7Pool := NewLoadBalancerPool(
		fakeLbs,
		&compute.BackendService{
			SelfLink: defaultBeName,
		},
	)
	healthChecks, err := NewHealthChecker(fakeHcs, defaultHttpHealthCheck, "/")
	if err != nil {
		return nil, err
	}
	cm := &ClusterManager{
		ClusterName:   clusterName,
		instancePool:  nodePool,
		backendPool:   backendPool,
		l7Pool:        l7Pool,
		healthChecker: healthChecks,
	}
	return &fakeClusterManager{cm, fakeLbs, fakeBackends, fakeIgs}, nil
}

type testIP struct {
	start int
}

func (t *testIP) ip() string {
	t.start++
	return fmt.Sprintf("0.0.0.%v", t.start)
}
