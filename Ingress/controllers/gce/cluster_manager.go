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

	"k8s.io/kubernetes/pkg/cloudprovider"
	gce "k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

const (
	defaultPort            = 80
	defaultHealthCheckPath = "/"
	defaultPortRange       = "80"
	defaultPortName        = "default-port"
	defaultHttpHealthCheck = "k8s-default-health-check"

	// A single instance-group is created per cluster manager.
	// Tagged with the name of the controller.
	instanceGroupPrefix = "k8s-ig"

	// A backend is created per nodePort, tagged with the nodeport.
	// This allows sharing of backends across loadbalancers.
	backendPrefix = "k8s-be"

	// A single target proxy/urlmap/forwarding rule is created per loadbalancer.
	// Tagged with the namespace/name of the Ingress.
	targetProxyPrefix    = "k8s-tp"
	forwardingRulePrefix = "k8s-fw"
	urlMapPrefix         = "k8s-um"

	// The gce api uses the name of a path rule to match a host rule.
	hostRulePrefix = "host"

	// State string required by gce library to list all instances.
	allInstances = "ALL"

	// Used in the test RunServer method to denote a delete request.
	deleteType = "del"

	// port 0 is used as a signal for port not found/no such port etc.
	invalidPort = 0
)

// ClusterManager manages cluster resource pools.
type ClusterManager struct {
	ClusterName            string
	defaultBackendNodePort int64
	instancePool           NodePool
	backendPool            BackendPool
	l7Pool                 LoadBalancerPool
	// We currently only employ a single cluster wide health check
	healthChecker HealthChecker
}

func (c *ClusterManager) shutdown() error {
	if err := c.l7Pool.Shutdown(); err != nil {
		return err
	}
	if err := c.backendPool.Shutdown(); err != nil {
		return err
	}
	if err := c.instancePool.Shutdown(); err != nil {
		return err
	}
	return c.healthChecker.Delete(defaultHttpHealthCheck)
}

func defaultInstanceGroupName(clusterName string) string {
	return fmt.Sprintf("%v-%v", instanceGroupPrefix, clusterName)
}

func defaultBackendName(clusterName string) string {
	return fmt.Sprintf("%v-%v", backendPrefix, clusterName)
}

// NewClusterManager creates a cluster manager for shared resources.
// - name: is the name used to tag cluster wide shared resources. This is the
//   string passed to glbc via --gce-cluster-name.
// - defaultBackendNodePort: is the node port of glbc's default backend. This is
//	 the kubernetes Service that serves the 404 page if no urls match.
// - defaultHealthCheckPath: is the default path used for L7 health checks, eg: "/healthz"
func NewClusterManager(name string, defaultBackendNodePort int64, defaultHealthCheckPath string) (*ClusterManager, error) {
	cloudInterface, err := cloudprovider.GetCloudProvider("gce", nil)
	if err != nil {
		return nil, err
	}
	cloud := cloudInterface.(*gce.GCECloud)
	cluster := ClusterManager{ClusterName: name}

	// Why do we need so many defaults?
	// Default IG: We add all instances to a single ig, and
	// every service that requires loadbalancing opens up
	// a nodePort on the cluster, which translates to a node
	// on this default ig.
	//
	// Default Backend: We need a default backend to create
	// every urlmap, even if the user doesn't specify one.
	// This is the backend that gets requests if no paths match.
	// Note that this backend doesn't actually occupy a port
	// on the instance group.
	//
	// Default Health Check: The default backend used by an
	// Ingress that doesn't specify it.

	defaultIGName := defaultInstanceGroupName(name)
	if cluster.instancePool, err = NewNodePool(cloud, defaultIGName); err != nil {
		return nil, err
	}

	// TODO: We're roud tripping for a resource we just created.
	defaultIG, err := cluster.instancePool.Get(defaultIGName)
	if err != nil {
		return nil, err
	}
	if cluster.healthChecker, err = NewHealthChecker(
		cloud, defaultHttpHealthCheck, defaultHealthCheckPath); err != nil {
		return nil, err
	}
	defaultHc, err := cluster.healthChecker.Get(defaultHttpHealthCheck)
	if err != nil {
		return nil, err
	}
	if cluster.backendPool, err = NewBackendPool(
		cloud, defaultBackendNodePort, defaultIG, defaultHc, cloud); err != nil {
		return nil, err
	}
	cluster.defaultBackendNodePort = defaultBackendNodePort
	// TODO: Don't cast, the problem here is the default backend doesn't have
	// a port and the interface only allows backend access via port.
	cluster.l7Pool = NewLoadBalancerPool(
		cloud, cluster.backendPool.(*Backends).defaultBackend)
	return &cluster, nil
}
