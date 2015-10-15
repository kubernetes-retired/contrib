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
)

// HealthChecks manages health checks. Currently it is only capable of
// managing a single health check.
type HealthChecks struct {
	cloud SingleHealthCheck
	// default path for the health check, eg: '/'
	defaultPath string
}

// NewHealthChecker returns a HealthChecker.
// - s: implements SingleHealthCheck, used to create health checks in the cloud.
// - name: The name of this health check, if a health check with the same name
//   already exists, it will be reused.
// - defaultPath: The default path to use for the health check. The backend
//   service must serve a 200 page on this path.
func NewHealthChecker(s SingleHealthCheck, name string, defaultPath string) (*HealthChecks, error) {
	d := defaultPath
	if defaultPath != "" {
		d = defaultHealthCheckPath
	}
	hc := &HealthChecks{s, d}
	if err := hc.Add(name); err != nil {
		return nil, err
	}
	return hc, nil
}

// Add adds a healthcheck if one with the same name doesn't already exist.
func (h *HealthChecks) Add(name string) error {
	hc, _ := h.Get(name)
	if hc == nil {
		glog.Infof("Creating health check %v", name)
		if err := h.cloud.CreateHttpHealthCheck(
			&compute.HttpHealthCheck{
				Name:        name,
				Port:        defaultPort,
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

// Delete deletes the health check by name.
func (h *HealthChecks) Delete(name string) error {
	return h.cloud.DeleteHttpHealthCheck(name)
}

// Get returns the given health check.
func (h *HealthChecks) Get(name string) (*compute.HttpHealthCheck, error) {
	return h.cloud.GetHttpHealthCheck(name)
}
