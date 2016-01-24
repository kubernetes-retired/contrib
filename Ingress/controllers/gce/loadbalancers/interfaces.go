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
	compute "google.golang.org/api/compute/v1"
)

// LoadBalancers is an interface for managing all the gce resources needed by L7
// loadbalancers. We don't have individual pools for each of these resources
// because none of them are usable (or acquirable) stand-alone, unlinke backends
// and instance groups. The dependency graph:
// ForwardingRule -> UrlMaps -> TargetProxies
type LoadBalancers interface {
	// Forwarding Rules
	GetGlobalForwardingRule(name string) (*compute.ForwardingRule, error)
	CreateGlobalForwardingRule(proxy *compute.TargetHttpProxy, name string, portRange string) (*compute.ForwardingRule, error)
	DeleteGlobalForwardingRule(name string) error
	SetProxyForGlobalForwardingRule(fw *compute.ForwardingRule, proxy *compute.TargetHttpProxy) error

	// UrlMaps
	GetUrlMap(name string) (*compute.UrlMap, error)
	CreateUrlMap(backend *compute.BackendService, name string) (*compute.UrlMap, error)
	UpdateUrlMap(urlMap *compute.UrlMap) (*compute.UrlMap, error)
	DeleteUrlMap(name string) error

	// TargetProxies
	GetTargetHttpProxy(name string) (*compute.TargetHttpProxy, error)
	CreateTargetHttpProxy(urlMap *compute.UrlMap, name string) (*compute.TargetHttpProxy, error)
	DeleteTargetHttpProxy(name string) error
	SetUrlMapForTargetHttpProxy(proxy *compute.TargetHttpProxy, urlMap *compute.UrlMap) error
}

// LoadBalancerPool is an interface to manage the cloud resources associated
// with a gce loadbalancer.
type LoadBalancerPool interface {
	Get(name string) (*L7, error)
	Add(name string) error
	Delete(name string) error
	Sync(names []string) error
	GC(names []string) error
	Shutdown() error
}
