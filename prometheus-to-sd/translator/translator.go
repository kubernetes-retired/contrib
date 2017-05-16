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
	"math"
	"time"

	"github.com/golang/glog"
	dto "github.com/prometheus/client_model/go"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
)

const (
	// Built-in Prometheus metric exporting process start time.
	processStartTimeMetric = "process_start_time_seconds"
)

// TranslatePrometheusToStackdriver translates metrics in Prometheus format to Stackdriver format.
func TranslatePrometheusToStackdriver(config *config.GceConfig, component string, metrics map[string]*dto.MetricFamily, whitelisted []string) []*v3.TimeSeries {
	// For cumulative metrics we need to know process start time.
	var startTime time.Time
	if family, found := metrics[processStartTimeMetric]; found && family.GetType() == dto.MetricType_GAUGE && len(family.GetMetric()) == 1 {
		startSec := family.Metric[0].Gauge.Value
		startTime = time.Unix(int64(*startSec), 0)
		glog.V(4).Infof("Monitored process start time: %v", startTime)
	} else {
		glog.Warningf("Metric %s invalid or not defined. Using %v instead. Cumulative metrics might be inaccurate.", processStartTimeMetric, startTime)
	}
	endTime := time.Now()

	if len(whitelisted) > 0 {
		metrics = filterWhitelisted(metrics, whitelisted)
	}

	var ts []*v3.TimeSeries
	for name, metric := range metrics {
		t, err := translateFamily(config, component, metric, startTime, endTime)
		if err != nil {
			glog.Warningf("Error while processing metric %s: %v", name, err)
		} else {
			ts = append(ts, t...)
		}
	}
	return ts
}

func filterWhitelisted(allMetrics map[string]*dto.MetricFamily, whitelisted []string) map[string]*dto.MetricFamily {
	glog.V(4).Infof("Exporting only whitelisted metrics: %v", whitelisted)
	res := map[string]*dto.MetricFamily{}
	for _, w := range whitelisted {
		if family, found := allMetrics[w]; found {
			res[w] = family
		} else {
			glog.V(4).Infof("Whitelisted metric %s not present in Prometheus endpoint.", w)
		}
	}
	return res
}

func translateFamily(config *config.GceConfig, component string, family *dto.MetricFamily, start, end time.Time) ([]*v3.TimeSeries, error) {
	glog.V(4).Infof("Translating metric family %v", family.GetName())
	var ts []*v3.TimeSeries
	if family.GetType() != dto.MetricType_COUNTER && family.GetType() != dto.MetricType_GAUGE && family.GetType() != dto.MetricType_HISTOGRAM {
		return ts, fmt.Errorf("Metric type %v of family %s not supported", family.GetType(), family.GetName())
	}
	for _, metric := range family.GetMetric() {
		t := translateOne(config, component, family.GetName(), family.GetType(), metric, start, end)
		ts = append(ts, t)
		glog.V(4).Infof("%+v\nMetric: %+v, Interval: %+v", *t, *(t.Metric), t.Points[0].Interval)
	}
	return ts, nil
}

// assumes that mType is Counter, Gauge or Histogram
func translateOne(config *config.GceConfig, component string, name string, mType dto.MetricType, metric *dto.Metric, start, end time.Time) *v3.TimeSeries {
	fullName := fmt.Sprintf("%s/%s/%s", config.MetricsPrefix, component, name)

	metricKind := "GAUGE"
	interval := &v3.TimeInterval{
		EndTime: end.UTC().Format(time.RFC3339),
	}
	if mType == dto.MetricType_COUNTER || mType == dto.MetricType_HISTOGRAM {
		metricKind = "CUMULATIVE"
		interval.StartTime = start.UTC().Format(time.RFC3339)
	}

	valueType := "INT64"
	if mType == dto.MetricType_HISTOGRAM {
		valueType = "DISTRIBUTION"
	}

	point := &v3.Point{
		Interval: interval,
		Value: &v3.TypedValue{
			ForceSendFields: []string{},
		},
	}
	setValue(mType, metric, point)

	return &v3.TimeSeries{
		Metric: &v3.Metric{
			Labels: getMetricLabels(metric.GetLabel()),
			Type:   fullName,
		},
		Resource: &v3.MonitoredResource{
			Labels: getResourceLabels(config),
			Type:   "gke_container",
		},
		MetricKind: metricKind,
		ValueType:  valueType,
		Points:     []*v3.Point{point},
	}
}

func setValue(mType dto.MetricType, metric *dto.Metric, point *v3.Point) {
	if mType == dto.MetricType_GAUGE {
		val := int64(metric.GetGauge().GetValue())
		point.Value.Int64Value = &val
		point.ForceSendFields = append(point.ForceSendFields, "Int64Value")
	} else if mType == dto.MetricType_HISTOGRAM {
		point.Value.DistributionValue = getHistogramValue(metric.GetHistogram())
		point.ForceSendFields = append(point.ForceSendFields, "DistributionValue")
	} else {
		val := int64(metric.GetCounter().GetValue())
		point.Value.Int64Value = &val
		point.ForceSendFields = append(point.ForceSendFields, "Int64Value")
	}
}

func getHistogramValue(h *dto.Histogram) *v3.Distribution {
	count := int64(h.GetSampleCount())
	mean := float64(0)
	dev := float64(0)
	bounds := []float64{}
	values := []int64{}

	if count > 0 {
		mean = h.GetSampleSum() / float64(count)
	}

	prevVal := uint64(0)
	lower := float64(0)
	for _, b := range h.Bucket {
		upper := b.GetUpperBound()
		if !math.IsInf(b.GetUpperBound(), 1) {
			bounds = append(bounds, b.GetUpperBound())
		} else {
			upper = lower
		}
		val := b.GetCumulativeCount() - prevVal
		x := (lower + upper) / float64(2)
		dev += float64(val) * (x - mean) * (x - mean)

		values = append(values, int64(b.GetCumulativeCount()-prevVal))

		lower = b.GetUpperBound()
		prevVal = b.GetCumulativeCount()
	}

	return &v3.Distribution{
		Count: count,
		Mean:  mean,
		SumOfSquaredDeviation: dev,
		BucketOptions: &v3.BucketOptions{
			ExplicitBuckets: &v3.Explicit{
				Bounds: bounds,
			},
		},
		BucketCounts: values,
	}
}

func getMetricLabels(labels []*dto.LabelPair) map[string]string {
	metricLabels := map[string]string{}
	for _, label := range labels {
		metricLabels[label.GetName()] = label.GetValue()
	}
	return metricLabels
}

func getResourceLabels(config *config.GceConfig) map[string]string {
	return map[string]string{
		"project_id":     config.Project,
		"cluster_name":   config.Cluster,
		"zone":           config.Zone,
		"instance_id":    config.Instance,
		"namespace_id":   "",
		"pod_id":         "machine",
		"container_name": "",
	}
}
