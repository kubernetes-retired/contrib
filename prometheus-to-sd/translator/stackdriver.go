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
	"fmt"
	"sync"

	"github.com/golang/glog"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
	"strings"
)

const (
	maxTimeseriesesPerRequest = 200
)

// SendToStackdriver sends http request to Stackdriver to create the given timeserieses.
func SendToStackdriver(service *v3.Service, config *config.GceConfig, ts []*v3.TimeSeries) {
	if len(ts) == 0 {
		glog.Warningf("No metrics to send to Stackdriver")
		return
	}

	proj := fmt.Sprintf("projects/%s", config.Project)

	var wg sync.WaitGroup
	for i := 0; i < len(ts); i += maxTimeseriesesPerRequest {
		end := i + maxTimeseriesesPerRequest
		if end > len(ts) {
			end = len(ts)
		}
		wg.Add(1)
		go func(begin int, end int) {
			defer wg.Done()
			req := &v3.CreateTimeSeriesRequest{TimeSeries: ts[begin:end]}
			_, err := service.Projects.TimeSeries.Create(proj, req).Do()
			if err != nil {
				glog.Errorf("Error while sending request to Stackdriver %v", err)
			}
		}(i, end)
	}
	wg.Wait()
	glog.V(4).Infof("Successfully sent %v timeserieses to Stackdriver", len(ts))
}

// GetMetricType formats a Metric.Type for given component and metric names.
func GetMetricType(config *config.GceConfig, component string, metricName string) string {
	return fmt.Sprintf("%s/%s/%s", config.MetricsPrefix, component, metricName)
}

// ParseMetricType extracts component and metricName from Metric.Type (e.g. output of GetMetricType).
func ParseMetricType(config *config.GceConfig, metricType string) (component, metricName string, err error) {
	if !strings.HasPrefix(metricType, config.MetricsPrefix) {
		return "", "", fmt.Errorf("MetricType is expected to have prefix: %v. Got %v instead.", config.MetricsPrefix, metricType)
	}

	componentMetricName := strings.TrimPrefix(metricType, fmt.Sprintf("%s/", config.MetricsPrefix))
	split := strings.SplitN(componentMetricName, "/", 2)

	if len(split) != 2 {
		return "", "", fmt.Errorf("MetricType should be in format %v/<component>/<name>. Got %v instead.", config.MetricsPrefix, metricType)
	}

	return split[0], split[1], nil
}

// GetMetricDescriptors fetches all metric descriptors of all metrics defined for given component.
func GetMetricDescriptors(service *v3.Service, config *config.GceConfig, component string) (map[string]*v3.MetricDescriptor, error) {
	proj := fmt.Sprintf("projects/%s", config.Project)

	metrics := make(map[string]*v3.MetricDescriptor)

	fn := func(page *v3.ListMetricDescriptorsResponse) error {
		for _, metricDescriptor := range page.MetricDescriptors {
			if _, metricName, err := ParseMetricType(config, metricDescriptor.Type); err == nil {
				metrics[metricName] = metricDescriptor
			} else {
				glog.Warningf("Unable to parse %v: %v", metricDescriptor.Type, err)
			}
		}

		return nil
	}

	filter := fmt.Sprintf("metric.type = starts_with(\"%s/%s\")", config.MetricsPrefix, component)

	return metrics, service.Projects.MetricDescriptors.List(proj).Filter(filter).Pages(nil, fn)
}
