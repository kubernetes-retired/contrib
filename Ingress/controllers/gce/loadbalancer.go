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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
)

const (
	// This is the key used to transmit the defaultBackend through a urlmap. It's
	// not a valid subdomain, and it is a catch all path.
	// TODO: Find a better way to transmit this, once we've decided on default
	// backend semantics (i.e do we want a default per host, per lb etc).
	defaultBackendKey = "DefaultBackend"

	// The host used if none is specified. It is a valid value for Host
	// recognized by GCE.
	defaultHost = "*"
)

// gceUrlMap is a nested map of hostname->path regex->backend
type gceUrlMap map[string]map[string]*compute.BackendService

// getDefaultBackend performs a destructive read and returns the default
// backend of the urlmap.
func (g gceUrlMap) getDefaultBackend() *compute.BackendService {
	var d *compute.BackendService
	var exists bool
	if h, ok := g[defaultBackendKey]; ok {
		if d, exists = h[defaultBackendKey]; exists {
			delete(h, defaultBackendKey)
		}
		delete(g, defaultBackendKey)
	}
	return d
}

// String implements the string interface for the gceUrlMap.
func (g gceUrlMap) String() string {
	msg := ""
	for host, um := range g {
		msg += fmt.Sprintf("%v\n", host)
		for url, be := range um {
			msg += fmt.Sprintf("\t%v: ", url)
			if be == nil {
				msg += fmt.Sprintf("No backend\n")
			} else {
				msg += fmt.Sprintf("%v\n", be.Name)
			}
		}
	}
	return msg
}

// putDefaultBackend performs a destructive write replacing the
// default backend of the url map with the given backend.
func (g gceUrlMap) putDefaultBackend(d *compute.BackendService) {
	g[defaultBackendKey] = map[string]*compute.BackendService{
		defaultBackendKey: d,
	}
}

// L7s implements LoadBalancerPool.
type L7s struct {
	cloud          LoadBalancers
	pool           *poolStore
	defaultBackend *compute.BackendService
}

// NewLoadBalancerPool returns a new loadbalancer pool.
// - cloud: implements LoadBalancers. Used to sync L7 loadbalancer resources
//	 with the cloud.
// - defaultBe: a GCE BackendService used as the default backend for
//   loadbalancers that don't specify one. This BackendService should point to
//   the NodePort of a kubernetes service capable of serving a 404 page.
func NewLoadBalancerPool(
	cloud LoadBalancers,
	defaultBe *compute.BackendService) LoadBalancerPool {
	return &L7s{cloud, newPoolStore(), defaultBe}
}

func (l *L7s) create(name string) (*L7, error) {
	return &L7{
		Name:           name,
		cloud:          l.cloud,
		defaultBackend: l.defaultBackend,
	}, nil
}

func lbName(key string) string {
	return strings.Replace(key, "/", "-", -1)
}

// Get returns the loadbalancer by name.
func (l *L7s) Get(name string) (*L7, error) {
	name = lbName(name)
	lb, exists := l.pool.Get(name)
	if !exists {
		return nil, fmt.Errorf("Loadbalancer %v not in pool", name)
	}
	return lb.(*L7), nil
}

// Add gets or creates a loadbalancer.
// If the loadbalancer already exists, it checks that its edges are valid.
func (l *L7s) Add(name string) (err error) {
	name = lbName(name)
	lb, _ := l.Get(name)
	if lb == nil {
		glog.Infof("Creating l7 %v", name)
		lb, err = l.create(name)
		if err != nil {
			return err
		}
	}
	// Why edge hop for the create?
	// The loadbalancer is a fictitious resource, it doesn't exist in gce. To
	// make it exist we need to create a collection of gce resources, done
	// through the edge hop.
	if err := lb.edgeHop(); err != nil {
		return err
	}

	l.pool.Add(name, lb)
	return nil
}

// Delete deletes a loadbalancer by name.
func (l *L7s) Delete(name string) error {
	name = lbName(name)
	lb, err := l.Get(name)
	if err != nil {
		return err
	}
	glog.Infof("Deleting lb %v", lb.Name)
	if err := lb.Cleanup(); err != nil {
		return err
	}
	l.pool.Delete(lb.Name)
	return nil
}

// Sync loadbalancers with the given names.
func (l *L7s) Sync(names []string) error {
	glog.Infof("Creating loadbalancers %+v", names)

	// create new loadbalancers, perform an edge hop for existing
	for _, n := range names {
		if err := l.Add(n); err != nil {
			return err
		}
	}
	return nil
}

// GC garbage collects loadbalancers not in the input list.
func (l *L7s) GC(names []string) error {
	knownLoadBalancers := sets.NewString()
	for _, n := range names {
		knownLoadBalancers.Insert(lbName(n))
	}
	pool := l.pool.snapshot()

	// Delete unknown loadbalancers
	for name := range pool {
		if knownLoadBalancers.Has(name) {
			continue
		}
		glog.Infof("GCing loadbalancer %v", name)
		if err := l.Delete(name); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown logs whether or not the pool is empty.
func (l *L7s) Shutdown() error {
	if err := l.GC([]string{}); err != nil {
		return err
	}
	glog.Infof("Loadbalancer pool shutdown.")
	return nil
}

// L7 represents a single L7 loadbalancer.
type L7 struct {
	Name  string
	cloud LoadBalancers
	um    *compute.UrlMap
	tp    *compute.TargetHttpProxy
	fw    *compute.ForwardingRule
	// This is the backend to use if no path rules match
	// TODO: Expose this to users.
	defaultBackend *compute.BackendService
}

func (l *L7) checkUrlMap(backend *compute.BackendService) (err error) {
	if l.defaultBackend == nil {
		return fmt.Errorf("Cannot create urlmap without default backend.")
	}
	urlMapName := fmt.Sprintf("%v-%v", urlMapPrefix, l.Name)
	urlMap, _ := l.cloud.GetUrlMap(urlMapName)
	if urlMap != nil {
		glog.Infof("Url map %v already exists", urlMap.Name)
		l.um = urlMap
		return nil
	}

	glog.Infof("Creating url map %v for backend %v", urlMapName, l.defaultBackend.Name)
	urlMap, err = l.cloud.CreateUrlMap(l.defaultBackend, urlMapName)
	if err != nil {
		return err
	}
	l.um = urlMap
	return nil
}

func (l *L7) checkProxy() (err error) {
	if l.um == nil {
		return fmt.Errorf("Cannot create proxy without urlmap.")
	}
	proxyName := fmt.Sprintf("%v-%v", targetProxyPrefix, l.Name)
	proxy, _ := l.cloud.GetTargetHttpProxy(proxyName)
	if proxy == nil {
		glog.Infof("Creating new http proxy for urlmap %v", l.um.Name)
		proxy, err = l.cloud.CreateTargetHttpProxy(l.um, proxyName)
		if err != nil {
			return err
		}
		l.tp = proxy
		return nil
	}
	if !compareLinks(proxy.UrlMap, l.um.SelfLink) {
		glog.Infof("Proxy %v has the wrong url map, setting %v overwriting %v",
			proxy.Name, l.um, proxy.UrlMap)
		if err := l.cloud.SetUrlMapForTargetHttpProxy(proxy, l.um); err != nil {
			return err
		}
	}
	l.tp = proxy
	return nil
}

func (l *L7) checkForwardingRule() (err error) {
	if l.tp == nil {
		return fmt.Errorf("Cannot create forwarding rule without proxy.")
	}

	forwardingRuleName := fmt.Sprintf("%v-%v", forwardingRulePrefix, l.Name)
	fw, _ := l.cloud.GetGlobalForwardingRule(forwardingRuleName)
	if fw == nil {
		glog.Infof("Creating forwarding rule for proxy %v", l.tp.Name)
		fw, err = l.cloud.CreateGlobalForwardingRule(
			l.tp, forwardingRuleName, defaultPortRange)
		if err != nil {
			return err
		}
		l.fw = fw
		return nil
	}
	// TODO: If the port range and protocol don't match, recreate the rule
	if compareLinks(fw.Target, l.tp.SelfLink) {
		glog.Infof("Forwarding rule %v already exists", fw.Name)
		l.fw = fw
		return nil
	}
	glog.Infof("Forwarding rule %v has the wrong proxy, setting %v overwriting %v",
		fw.Name, fw.Target, l.tp.SelfLink)
	if err := l.cloud.SetProxyForGlobalForwardingRule(fw, l.tp); err != nil {
		return err
	}
	l.fw = fw
	return nil
}

func (l *L7) edgeHop() error {
	if err := l.checkUrlMap(l.defaultBackend); err != nil {
		return err
	}
	if err := l.checkProxy(); err != nil {
		return err
	}
	if err := l.checkForwardingRule(); err != nil {
		return err
	}
	return nil
}

// GetIP returns the ip associated with the forwarding rule for this l7.
func (l *L7) GetIP() string {
	return l.fw.IPAddress
}

// getNameForPathMatcher returns a name for a pathMatcher based on the given host rule.
// The host rule can be a regex, the path matcher name used to associate the 2 cannot.
func getNameForPathMatcher(hostRule string) string {
	hasher := md5.New()
	hasher.Write([]byte(hostRule))
	return fmt.Sprintf("%v%v", hostRulePrefix, hex.EncodeToString(hasher.Sum(nil)))
}

// UpdateUrlMap translates the given hostname: endpoint->port mapping into a gce url map.
//
// HostRule: Conceptually contains all PathRules for a given host.
// PathMatcher: Associates a path rule with a host rule. Mostly an optimization.
// PathRule: Maps a single path regex to a backend.
//
// The GCE url map allows multiple hosts to share url->backend mappings without duplication, eg:
//   Host: foo(PathMatcher1), bar(PathMatcher1,2)
//   PathMatcher1:
//     /a -> b1
//     /b -> b2
//   PathMatcher2:
//     /c -> b1
// This leads to a lot of complexity in the common case, where all we want is a mapping of
// host->{/path: backend}.
//
// Consider some alternatives:
// 1. Using a single backend per PathMatcher:
//   Host: foo(PathMatcher1,3) bar(PathMatcher1,2,3)
//   PathMatcher1:
//     /a -> b1
//   PathMatcher2:
//     /c -> b1
//   PathMatcher3:
//     /b -> b2
// 2. Using a single host per PathMatcher:
//   Host: foo(PathMatcher1)
//   PathMatcher1:
//     /a -> b1
//     /b -> b2
//   Host: bar(PathMatcher2)
//   PathMatcher2:
//     /a -> b1
//     /b -> b2
//     /c -> b1
// In the context of kubernetes services, 2 makes more sense, because we
// rarely want to lookup backends (service:nodeport). When a service is
// deleted, we need to find all host PathMatchers that have the backend
// and remove the mapping. When a new path is added to a host (happens
// more frequently than service deletion) we just need to lookup the 1
// pathmatcher of the host.
func (l *L7) UpdateUrlMap(ingressRules gceUrlMap) error {
	if l.um == nil {
		return fmt.Errorf("Cannot add url without an urlmap.")
	}
	glog.Infof("Updating urlmap for l7 %v", l.Name)

	defaultService := l.um.DefaultService
	defaultBackend := ingressRules.getDefaultBackend()

	// If the Ingress has a default backend, it applies to all host rules as
	// well as to the urlmap itself. If it doesn't the urlmap might have a
	// stale default, so replace it with the controller's default backend.
	if defaultBackend != nil {
		defaultService = defaultBackend.SelfLink
		l.um.DefaultService = defaultService
	} else {
		l.um.DefaultService = l.defaultBackend.SelfLink
	}
	glog.V(3).Infof("Updating url map %+v", ingressRules)

	for hostname, urlToBackend := range ingressRules {
		// Find the hostrule
		// Find the path matcher
		// Add all given endpoint:backends to pathRules in path matcher
		var hostRule *compute.HostRule
		pmName := getNameForPathMatcher(hostname)
		for _, hr := range l.um.HostRules {
			// TODO: Hostnames must be exact match?
			if hr.Hosts[0] == hostname {
				hostRule = hr
				break
			}
		}
		if hostRule == nil {
			// This is a new host
			hostRule = &compute.HostRule{
				Hosts:       []string{hostname},
				PathMatcher: pmName,
			}
			// Why not just clobber existing host rules?
			// Because we can have multiple loadbalancers point to a single
			// gce url map when we have IngressClaims.
			l.um.HostRules = append(l.um.HostRules, hostRule)
		}
		var pathMatcher *compute.PathMatcher
		for _, pm := range l.um.PathMatchers {
			if pm.Name == hostRule.PathMatcher {
				pathMatcher = pm
				break
			}
		}
		if pathMatcher == nil {
			// This is a dangling or new host
			pathMatcher = &compute.PathMatcher{Name: pmName}
			l.um.PathMatchers = append(l.um.PathMatchers, pathMatcher)
		}
		pathMatcher.DefaultService = defaultService

		// TODO: Every update replaces the entire path map. This will need to
		// change when we allow joining. Right now we call a single method
		// to verify current == desired and add new url mappings.
		pathMatcher.PathRules = []*compute.PathRule{}

		// Longest prefix wins. For equal rules, first hit wins, i.e the second
		// /foo rule when the first is deleted.
		for expr, be := range urlToBackend {
			pathMatcher.PathRules = append(
				pathMatcher.PathRules, &compute.PathRule{[]string{expr}, be.SelfLink})
		}
	}
	um, err := l.cloud.UpdateUrlMap(l.um)
	if err != nil {
		return err
	}
	l.um = um
	return nil
}

// Cleanup deletes resources specific to this l7 in the right order.
// forwarding rule -> target proxy -> url map
// This leaves backends and health checks, which are shared across loadbalancers.
func (l *L7) Cleanup() error {
	if l.fw != nil {
		glog.Infof("Deleting global forwarding rule %v", l.fw.Name)
		if err := l.cloud.DeleteGlobalForwardingRule(l.fw.Name); err != nil {
			if !isHTTPErrorCode(err, http.StatusNotFound) {
				return err
			}
		}
		l.fw = nil
	}
	if l.tp != nil {
		glog.Infof("Deleting target proxy %v", l.tp.Name)
		if err := l.cloud.DeleteTargetHttpProxy(l.tp.Name); err != nil {
			if !isHTTPErrorCode(err, http.StatusNotFound) {
				return err
			}
		}
		l.tp = nil
	}
	if l.um != nil {
		glog.Infof("Deleting url map %v", l.um.Name)
		if err := l.cloud.DeleteUrlMap(l.um.Name); err != nil {
			if !isHTTPErrorCode(err, http.StatusNotFound) {
				return err
			}
		}
		l.um = nil
	}
	return nil
}

// getBackendNames returns the names of backends in this L7 urlmap.
func (l *L7) getBackendNames() []string {
	if l.um == nil {
		return []string{}
	}
	beNames := sets.NewString()
	for _, pathMatcher := range l.um.PathMatchers {
		for _, pathRule := range pathMatcher.PathRules {
			// This is gross, but the urlmap only has links to backend services.
			parts := strings.Split(pathRule.Service, "/")
			name := parts[len(parts)-1]
			if name != "" {
				beNames.Insert(name)
			}
		}
	}
	return beNames.List()
}
