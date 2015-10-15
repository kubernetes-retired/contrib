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
	"strconv"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
)

// Backends implements BackendPool.
type Backends struct {
	cloud          BackendServices
	instanceGroups InstanceGroups
	pool           *poolStore
	defaultIG      *compute.InstanceGroup
	defaultBackend *compute.BackendService
	defaultHc      *compute.HttpHealthCheck
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
// - defaultIG: is the GCE Instance Group that contains all the vms in your
//	 cluster. Each new backend opens a port on this Instance Group.
// - defaultHc: is default GCE health check to use for all backends.
// - instanceGroups: implements InstanceGroups, every new backend uses this
//   interface to open a port for itself on the defaultIG.
func NewBackendPool(
	cloud BackendServices,
	defaultBackendNodePort int64,
	defaultIG *compute.InstanceGroup,
	defaultHc *compute.HttpHealthCheck,
	instanceGroups InstanceGroups) (BackendPool, error) {
	backends := &Backends{
		cloud:          cloud,
		instanceGroups: instanceGroups,
		pool:           newPoolStore(),
		defaultIG:      defaultIG,
		defaultHc:      defaultHc,
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
		HealthChecks: []string{b.defaultHc.SelfLink},
		Port:         namedPort.Port,
		PortName:     namedPort.Name,
	}
	if err := b.cloud.CreateBackendService(backend); err != nil {
		return nil, err
	}
	return b.cloud.GetBackendService(name)
}

// Add will get or create a Backend for the given port.
// If a backend already exists, it performs an edgehop.
// If one doesn't already exist, it will create it.
// If the port isn't one of the named ports in the instance group,
// it will add it. It returns a backend ready for insertion into a
// urlmap.
func (b *Backends) Add(port int64) error {
	namedPort, err := b.instanceGroups.AddPortToInstanceGroup(
		b.defaultIG, port)
	if err != nil {
		return err
	}
	be, _ := b.Get(port)
	if be == nil {
		glog.Infof("Creating backend for instance group %v port %v named port %v",
			b.defaultIG.Name, port, namedPort)
		_, err = b.create(b.defaultIG, namedPort, beName(port))
		if err != nil {
			return err
		}
	} else {
		glog.Infof("Backend %v already exists", be.Name)
		if err := b.edgeHop(be); err != nil {
			return err
		}
	}
	_, err = b.Get(port)
	return err
}

// Delete deletes the Backend for the given port.
func (b *Backends) Delete(port int64) error {
	name := beName(port)
	glog.Infof("Deleting backend %v", name)
	if err := b.cloud.DeleteBackendService(name); err != nil {
		return err
	}
	b.pool.Delete(portKey(port))
	return nil
}

// edgeHop checks the links of the given backend by executing an edge hop.
// It fixes broken links.
func (b *Backends) edgeHop(be *compute.BackendService) error {
	if len(be.Backends) == 1 &&
		compareLinks(be.Backends[0].Group, b.defaultIG.SelfLink) {
		return nil
	}
	glog.Infof("Backend %v has a broken edge, adding link to %v",
		be.Name, b.defaultIG.Name)
	be.Backends = []*compute.Backend{
		{Group: b.defaultIG.SelfLink},
	}
	if err := b.cloud.UpdateBackendService(be); err != nil {
		return err
	}
	return nil
}

// Sync syncs backend services corresponding to ports in the given list.
func (b *Backends) Sync(svcNodePorts []int64) error {
	glog.Infof("Sync: backends %v", svcNodePorts)
	// create backends for new ports, perform an edge hop for existing ports
	for _, port := range svcNodePorts {
		if err := b.Add(port); err != nil {
			return err
		}
	}

	// The default backend isn't part of the nodeports given
	return b.edgeHop(b.defaultBackend)
}

// GC garbage collects services corresponding to ports in the given list.
func (b *Backends) GC(svcNodePorts []int64) error {
	glog.Infof("GC: Existing backends %v", svcNodePorts)

	knownPorts := sets.NewString()
	for _, port := range svcNodePorts {
		knownPorts.Insert(portKey(port))
	}
	pool := b.pool.snapshot()

	// gc unknown ports
	for port, be := range pool {
		p, err := strconv.Atoi(port)
		if err != nil {
			return err
		}
		if knownPorts.Has(portKey(int64(p))) ||
			compareLinks(be.(*compute.BackendService).SelfLink,
				b.defaultBackend.SelfLink) {
			continue
		}
		if err := b.Delete(int64(p)); err != nil {
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
	if err := b.cloud.DeleteBackendService(b.defaultBackend.Name); err != nil {
		return err
	}
	return nil
}
