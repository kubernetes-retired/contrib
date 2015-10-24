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
	compute "google.golang.org/api/compute/v1"

	"github.com/golang/glog"
	"net/http"
)

// HealthChecks manages health checks.
type HealthChecks struct {
	cloud       SingleHealthCheck
	defaultPath string
}

// NewHealthChecker creates a new health checker.
// cloud: the cloud object implementing SingleHealthCheck.
// defaultHealthCheckPath: is the HTTP path to use for health checks.
func NewHealthChecker(cloud SingleHealthCheck, defaultHealthCheckPath string) HealthChecker {
	return &HealthChecks{cloud, defaultHealthCheckPath}
}

// Add adds a healthcheck if one for the same port doesn't already exist.
func (h *HealthChecks) Add(port int64) error {
	hc, _ := h.Get(port)
	name := beName(port)
	if hc == nil {
		glog.Infof("Creating health check %v", name)
		if err := h.cloud.CreateHttpHealthCheck(
			&compute.HttpHealthCheck{
				Name:        name,
				Port:        port,
				RequestPath: h.defaultPath,
				Description: "Default kubernetes L7 Loadbalancing health check.",
				// How often to health check.
				CheckIntervalSec: 1,
				// How long to wait before claiming failure of a health check.
				TimeoutSec: 1,
				// Number of healthchecks to pass for a vm to be deemed healthy.
				HealthyThreshold: 1,
				// Number of healthchecks to fail before the vm is deemed unhealthy.
				UnhealthyThreshold: 10,
			}); err != nil {
			return err
		}
	} else {
		// TODO: Does this health check need an edge hop?
		glog.Infof("Health check %v already exists", hc.Name)
	}
	return nil
}

// Delete deletes the health check by port.
func (h *HealthChecks) Delete(port int64) error {
	name := beName(port)
	glog.Infof("Deleting health check %v", name)
	if err := h.cloud.DeleteHttpHealthCheck(beName(port)); err != nil {
		if !isHTTPErrorCode(err, http.StatusNotFound) {
			return err
		}
	}
	return nil
}

// Get returns the given health check.
func (h *HealthChecks) Get(port int64) (*compute.HttpHealthCheck, error) {
	return h.cloud.GetHttpHealthCheck(beName(port))
}
