/*
Copyright 2017 The Kubernetes Authors.

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

package translator

import (
	"github.com/golang/glog"
	dto "github.com/prometheus/client_model/go"
	v3 "google.golang.org/api/monitoring/v3"

	"fmt"
	"k8s.io/contrib/prometheus-to-sd/config"
	"strings"
)

var (
	customMetricsPrefix = "custom.googleapis.com"
)

type MetricDescriptorCache struct {
	descriptors map[string]*v3.MetricDescriptor
	service     *v3.Service
	config      *config.GceConfig
	component   string
	fresh       bool
}

// NewMetricDescriptorCache creates empty metric descriptor cache for the given component.
func NewMetricDescriptorCache(service *v3.Service, config *config.GceConfig, component string) *MetricDescriptorCache {
	return &MetricDescriptorCache{
		descriptors: make(map[string]*v3.MetricDescriptor),
		service:     service,
		config:      config,
		component:   component,
		fresh:       false,
	}
}

// GetMetricNames returns a list of all metric names from the cache.
func (this *MetricDescriptorCache) GetMetricNames() []string {
	keys := make([]string, len(this.descriptors))
	for k := range this.descriptors {
		keys = append(keys, k)
	}
	return keys
}

// MarkStale makes function IsFresh to return false until the next cache refresh.
func (this *MetricDescriptorCache) MarkStale() {
	this.fresh = false
}

// UpdateMetricDescriptors iterates over all metricFamilies and updates metricDescriptors in the Stackdriver if required.
func (this *MetricDescriptorCache) UpdateMetricDescriptors(metrics map[string]*dto.MetricFamily) {
	// Perform this operation only if cache was recently refreshed. This is done mostly from the optimization point
	// of view, we don't want to check all metric descriptors too often, as they should change rarely.
	if !this.fresh {
		return
	}
	for _, metricFamily := range metrics {
		this.updateMetricDescriptorIfStale(metricFamily)
	}
}

// updateMetricDescriptorIfStale checks if descriptor created from MetricFamily object differs from the existing one
// and updates if needed.
func (this *MetricDescriptorCache) updateMetricDescriptorIfStale(metricFamily *dto.MetricFamily) {
	metricDescriptor, ok := this.descriptors[metricFamily.GetName()]
	updatedMetricDescriptor := MetricFamilyToMetricDescriptor(this.config, this.component, metricFamily)
	if strings.HasPrefix(updatedMetricDescriptor.Type, customMetricsPrefix) &&
		(!ok || descriptorChanged(metricDescriptor, updatedMetricDescriptor)) {
		this.updateMetricDescriptorInStackdriver(updatedMetricDescriptor)
		this.descriptors[metricFamily.GetName()] = updatedMetricDescriptor
	}
}

func descriptorChanged(original *v3.MetricDescriptor, checked *v3.MetricDescriptor) bool {
	if original.Description != checked.Description {
		glog.V(4).Infof("Description is different, %v != %v", original.Description, checked.Description)
		return true
	}
	return false
}

// updateMetricDescriptorInStackdriver writes metric descriptor to the stackdriver.
func (this *MetricDescriptorCache) updateMetricDescriptorInStackdriver(metricDescriptor *v3.MetricDescriptor) {
	glog.V(4).Infof("Updating metric descriptor: %+v", metricDescriptor)

	projectName := createProjectName(this.config)
	_, err := this.service.Projects.MetricDescriptors.Create(projectName, metricDescriptor).Do()
	if err != nil {
		glog.Errorf("Error in attempt to update metric descriptor %v", err)
	}
}

// Refresh function fetches all metric descriptors of all metrics defined for given component with a defined prefix
// and puts them into cache.
func (this *MetricDescriptorCache) Refresh() {
	proj := createProjectName(this.config)
	metrics := make(map[string]*v3.MetricDescriptor)
	fn := func(page *v3.ListMetricDescriptorsResponse) error {
		for _, metricDescriptor := range page.MetricDescriptors {
			if _, metricName, err := parseMetricType(this.config, metricDescriptor.Type); err == nil {
				metrics[metricName] = metricDescriptor
			} else {
				glog.Warningf("Unable to parse %v: %v", metricDescriptor.Type, err)
			}
		}
		return nil
	}
	filter := fmt.Sprintf("metric.type = starts_with(\"%s/%s\")", this.config.MetricsPrefix, this.component)
	err := this.service.Projects.MetricDescriptors.List(proj).Filter(filter).Pages(nil, fn)
	if err != nil {
		glog.Warningf("Error while fetching metric descriptors for %v: %v", this.component, err)
	}
	this.fresh = true
}
