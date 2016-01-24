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
	"fmt"
	"testing"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/contrib/Ingress/controllers/gce/utils"
)

var testIPManager = testIP{}

type testIP struct {
	start int
}

func (t *testIP) ip() string {
	t.start++
	return fmt.Sprintf("0.0.0.%v", t.start)
}

// Loadbalancer fakes

// FakeLoadBalancers is a type that fakes out the loadbalancer interface.
type FakeLoadBalancers struct {
	Fw   []*compute.ForwardingRule
	Um   []*compute.UrlMap
	Tp   []*compute.TargetHttpProxy
	name string
}

// TODO: There is some duplication between these functions and the name mungers in
// loadbalancer file.
func (f *FakeLoadBalancers) fwName() string {
	return fmt.Sprintf("%v-%v", forwardingRulePrefix, f.name)
}

func (f *FakeLoadBalancers) umName() string {
	return fmt.Sprintf("%v-%v", urlMapPrefix, f.name)
}

func (f *FakeLoadBalancers) tpName() string {
	return fmt.Sprintf("%v-%v", targetProxyPrefix, f.name)
}

// String is the string method for FakeLoadBalancers.
func (f *FakeLoadBalancers) String() string {
	msg := fmt.Sprintf(
		"Loadbalancer %v,\nforwarding rules:\n", f.name)
	for _, fw := range f.Fw {
		msg += fmt.Sprintf("\t%v\n", fw.Name)
	}
	msg += fmt.Sprintf("Target proxies\n")
	for _, tp := range f.Tp {
		msg += fmt.Sprintf("\t%v\n", tp.Name)
	}
	msg += fmt.Sprintf("UrlMaps\n")
	for _, um := range f.Um {
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

// Forwarding rule fakes

// GetGlobalForwardingRule returns a fake forwarding rule.
func (f *FakeLoadBalancers) GetGlobalForwardingRule(name string) (*compute.ForwardingRule, error) {
	for i := range f.Fw {
		if f.Fw[i].Name == name {
			return f.Fw[i], nil
		}
	}
	return nil, fmt.Errorf("Forwarding rule %v not found", name)
}

// CreateGlobalForwardingRule fakes forwarding rule creation.
func (f *FakeLoadBalancers) CreateGlobalForwardingRule(proxy *compute.TargetHttpProxy, name string, portRange string) (*compute.ForwardingRule, error) {
	rule := &compute.ForwardingRule{
		Name:       name,
		Target:     proxy.SelfLink,
		PortRange:  portRange,
		IPProtocol: "TCP",
		SelfLink:   f.fwName(),
		IPAddress:  fmt.Sprintf(testIPManager.ip()),
	}
	f.Fw = append(f.Fw, rule)
	return rule, nil
}

// SetProxyForGlobalForwardingRule fakes setting a global forwarding rule.
func (f *FakeLoadBalancers) SetProxyForGlobalForwardingRule(fw *compute.ForwardingRule, proxy *compute.TargetHttpProxy) error {
	for i := range f.Fw {
		if f.Fw[i].Name == fw.Name {
			f.Fw[i].Target = proxy.SelfLink
		}
	}
	return nil
}

// DeleteGlobalForwardingRule fakes deleting a global forwarding rule.
func (f *FakeLoadBalancers) DeleteGlobalForwardingRule(name string) error {
	fw := []*compute.ForwardingRule{}
	for i := range f.Fw {
		if f.Fw[i].Name != name {
			fw = append(fw, f.Fw[i])
		}
	}
	f.Fw = fw
	return nil
}

// UrlMaps fakes

// GetUrlMap fakes getting url maps from the cloud.
func (f *FakeLoadBalancers) GetUrlMap(name string) (*compute.UrlMap, error) {
	for i := range f.Um {
		if f.Um[i].Name == name {
			return f.Um[i], nil
		}
	}
	return nil, fmt.Errorf("Url Map %v not found", name)
}

// CreateUrlMap fakes url-map creation.
func (f *FakeLoadBalancers) CreateUrlMap(backend *compute.BackendService, name string) (*compute.UrlMap, error) {
	urlMap := &compute.UrlMap{
		Name:           name,
		DefaultService: backend.SelfLink,
		SelfLink:       f.umName(),
	}
	f.Um = append(f.Um, urlMap)
	return urlMap, nil
}

// UpdateUrlMap fakes updating url-maps.
func (f *FakeLoadBalancers) UpdateUrlMap(urlMap *compute.UrlMap) (*compute.UrlMap, error) {
	for i := range f.Um {
		if f.Um[i].Name == urlMap.Name {
			f.Um[i] = urlMap
			return urlMap, nil
		}
	}
	return nil, nil
}

// DeleteUrlMap fakes url-map deletion.
func (f *FakeLoadBalancers) DeleteUrlMap(name string) error {
	um := []*compute.UrlMap{}
	for i := range f.Um {
		if f.Um[i].Name != name {
			um = append(um, f.Um[i])
		}
	}
	f.Um = um
	return nil
}

// TargetProxies fakes

// GetTargetHttpProxy fakes getting target http proxies from the cloud.
func (f *FakeLoadBalancers) GetTargetHttpProxy(name string) (*compute.TargetHttpProxy, error) {
	for i := range f.Tp {
		if f.Tp[i].Name == name {
			return f.Tp[i], nil
		}
	}
	return nil, fmt.Errorf("Targetproxy %v not found", name)
}

// CreateTargetHttpProxy fakes creating a target http proxy.
func (f *FakeLoadBalancers) CreateTargetHttpProxy(urlMap *compute.UrlMap, name string) (*compute.TargetHttpProxy, error) {
	proxy := &compute.TargetHttpProxy{
		Name:     name,
		UrlMap:   urlMap.SelfLink,
		SelfLink: f.tpName(),
	}
	f.Tp = append(f.Tp, proxy)
	return proxy, nil
}

// DeleteTargetHttpProxy fakes deleting a target http proxy.
func (f *FakeLoadBalancers) DeleteTargetHttpProxy(name string) error {
	tp := []*compute.TargetHttpProxy{}
	for i := range f.Tp {
		if f.Tp[i].Name != name {
			tp = append(tp, f.Tp[i])
		}
	}
	f.Tp = tp
	return nil
}

// SetUrlMapForTargetHttpProxy fakes setting an url-map for a target http proxy.
func (f *FakeLoadBalancers) SetUrlMapForTargetHttpProxy(proxy *compute.TargetHttpProxy, urlMap *compute.UrlMap) error {
	for i := range f.Tp {
		if f.Tp[i].Name == proxy.Name {
			f.Tp[i].UrlMap = urlMap.SelfLink
		}
	}
	return nil
}

// CheckURLMap check a url-map for the expected rules.
func (f *FakeLoadBalancers) CheckURLMap(t *testing.T, l7 *L7, expectedMap map[string]utils.FakeIngressRuleValueMap) {
	um, err := f.GetUrlMap(l7.um.Name)
	if err != nil || um == nil {
		t.Fatalf("%v", err)
	}
	// Check the default backend
	var d string
	if h, ok := expectedMap[utils.DefaultBackendKey]; ok {
		if d, ok = h[utils.DefaultBackendKey]; ok {
			delete(h, utils.DefaultBackendKey)
		}
		delete(expectedMap, utils.DefaultBackendKey)
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

// NewFakeLoadBalancers creates a fake cloud client. Name is the name
// inserted into the selfLink of the associated resources for testing.
// eg: forwardingRule.SelfLink == k8-fw-name.
func NewFakeLoadBalancers(name string) *FakeLoadBalancers {
	return &FakeLoadBalancers{
		Fw:   []*compute.ForwardingRule{},
		name: name,
	}
}
