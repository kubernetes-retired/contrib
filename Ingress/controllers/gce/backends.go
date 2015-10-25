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
	"net/http"
	"strconv"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
)

// Backends implements BackendPool.
type Backends struct {
	cloud                  BackendServices
	nodePool               NodePool
	healthChecker          HealthChecker
	pool                   *poolStore
	defaultBackend         *compute.BackendService
	defaultBackendNodePort int64
}

func portKey(port int64) string {
	return fmt.Sprintf("%d", port)
}

func beName(port int64) string {
	return fmt.Sprintf("%v-%d", backendPrefix, port)
}

// NewBackendPool returns a new backend pool.
// - cloud: implements BackendServices and syncs backends with a cloud provider
// - defaultBackendNodePort: is the node port of glbc's default backend. This is
//	 the kubernetes Service that serves the 404 page if no urls match.
// - nodePool: implements NodePool, used to create/delete new instance groups.
func NewBackendPool(
	cloud BackendServices,
	defaultBackendNodePort int64,
	healthChecker HealthChecker,
	nodePool NodePool) (BackendPool, error) {
	backends := &Backends{
		cloud:                  cloud,
		nodePool:               nodePool,
		pool:                   newPoolStore(),
		healthChecker:          healthChecker,
		defaultBackendNodePort: defaultBackendNodePort,
	}
	// TODO: When we figure out a way to expose health checks, this will not
	// be necessary. The default backend must serve a 404 page on "/", which
	// is the most common 200 path for every other backend.
	if err := backends.healthChecker.Add(
		defaultBackendNodePort, "/healthz"); err != nil {
		return nil, err
	}
	err := backends.Add(defaultBackendNodePort)
	if err != nil {
		return nil, err
	}
	backends.defaultBackend, err = backends.Get(defaultBackendNodePort)
	if err != nil {
		return nil, err
	}
	return backends, nil
}

// Get returns a single backend.
func (b *Backends) Get(port int64) (*compute.BackendService, error) {
	be, err := b.cloud.GetBackendService(beName(port))
	if err != nil {
		return nil, err
	}
	b.pool.Add(portKey(port), be)
	return be, nil
}

func (b *Backends) create(ig *compute.InstanceGroup, namedPort *compute.NamedPort, name string) (*compute.BackendService, error) {
	// Create a new health check
	if err := b.healthChecker.Add(namedPort.Port, ""); err != nil {
		return nil, err
	}
	hc, err := b.healthChecker.Get(namedPort.Port)
	if err != nil {
		return nil, err
	}
	// Create a new backend
	backend := &compute.BackendService{
		Name:     name,
		Protocol: "HTTP",
		Backends: []*compute.Backend{
			{
				Group: ig.SelfLink,
			},
		},
		// Api expects one, means little to kubernetes.
		HealthChecks: []string{hc.SelfLink},
		Port:         namedPort.Port,
		PortName:     namedPort.Name,
	}
	if err := b.cloud.CreateBackendService(backend); err != nil {
		return nil, err
	}
	return b.Get(namedPort.Port)
}

// Add will get or create a Backend for the given port.
func (b *Backends) Add(port int64) error {
	name := beName(port)

	// Preemptive addition of a backend to the pool is only to force instance
	// cleanup if user runs out of quota mid-way through this function, and
	// decides to delete Ingress. A better solutions is to have the node pool
	// cleaup after itself.
	be := &compute.BackendService{}
	defer func() { b.pool.Add(portKey(port), be) }()

	ig, namedPort, err := b.nodePool.AddInstanceGroup(name, port)
	if err != nil {
		return err
	}
	be, _ = b.Get(port)
	if be == nil {
		glog.Infof("Creating backend for instance group %v port %v named port %v",
			ig.Name, port, namedPort)
		be, err = b.create(ig, namedPort, beName(port))
		if err != nil {
			return err
		}
	}
	// Both the backend and instance group might exist, but be disconnected.
	if err := b.edgeHop(be, ig); err != nil {
		return err
	}
	return err
}

// Delete deletes the Backend for the given port.
func (b *Backends) Delete(port int64) (err error) {
	name := beName(port)
	glog.Infof("Deleting backend %v", name)
	defer func() {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			err = nil
		}
		if err == nil {
			b.pool.Delete(portKey(port))
		}
	}()
	// Try deleting health checks and instance groups, even if a backend is
	// not found. This guards against the case where we create one of the
	// other 2 and run out of quota creating the backend.
	if err = b.cloud.DeleteBackendService(name); err != nil &&
		!isHTTPErrorCode(err, http.StatusNotFound) {
		return err
	}
	if err = b.healthChecker.Delete(port); err != nil &&
		!isHTTPErrorCode(err, http.StatusNotFound) {
		return err
	}
	glog.Infof("Deleting instance group %v", name)
	if err = b.nodePool.DeleteInstanceGroup(name); err != nil {
		return err
	}
	return nil
}

// edgeHop checks the links of the given backend by executing an edge hop.
// It fixes broken links.
func (b *Backends) edgeHop(be *compute.BackendService, ig *compute.InstanceGroup) error {
	if len(be.Backends) == 1 &&
		compareLinks(be.Backends[0].Group, ig.SelfLink) {
		return nil
	}
	glog.Infof("Backend %v has a broken edge, adding link to %v",
		be.Name, ig.Name)
	be.Backends = []*compute.Backend{
		{Group: ig.SelfLink},
	}
	if err := b.cloud.UpdateBackendService(be); err != nil {
		return err
	}
	return nil
}

// Sync syncs backend services corresponding to ports in the given list.
func (b *Backends) Sync(svcNodePorts []int64) error {
	glog.Infof("Sync: backends %v", svcNodePorts)

	// The default backend doesn't have an Ingress and won't be a part of the
	// input node port list.
	svcNodePorts = append(svcNodePorts, b.defaultBackendNodePort)

	// create backends for new ports, perform an edge hop for existing ports
	for _, port := range svcNodePorts {
		if err := b.Add(port); err != nil {
			return err
		}
	}
	return nil
}

// GC garbage collects services corresponding to ports in the given list.
func (b *Backends) GC(svcNodePorts []int64) error {
	knownPorts := sets.NewString()
	for _, port := range svcNodePorts {
		knownPorts.Insert(portKey(port))
	}
	pool := b.pool.snapshot()
	for port := range pool {
		p, err := strconv.Atoi(port)
		if err != nil {
			return err
		}
		nodePort := int64(p)
		if knownPorts.Has(portKey(nodePort)) ||
			nodePort == b.defaultBackendNodePort {
			continue
		}
		glog.Infof("GCing backend for port %v", p)
		if err := b.Delete(nodePort); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown deletes all backends and the default backend.
// This will fail if one of the backends is being used by another resource.
func (b *Backends) Shutdown() error {
	if err := b.GC([]int64{}); err != nil {
		return err
	}
	if err := b.Delete(b.defaultBackendNodePort); err != nil {
		return err
	}
	return nil
}

// Status returns the status of the given backend by name.
func (b *Backends) Status(name string) string {
	backend, err := b.cloud.GetBackendService(name)
	if err != nil {
		return "Unknown"
	}
	// TODO: Include port, ip in the status, since it's in the health info.
	hs, err := b.cloud.GetHealth(name, backend.Backends[0].Group)
	if err != nil || len(hs.HealthStatus) == 0 || hs.HealthStatus[0] == nil {
		return "Unknown"
	}
	// TODO: State transition are important, not just the latest.
	return hs.HealthStatus[0].HealthState
}
