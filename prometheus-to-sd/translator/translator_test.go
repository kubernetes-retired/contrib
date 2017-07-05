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
	"math"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	v3 "google.golang.org/api/monitoring/v3"
	"k8s.io/contrib/prometheus-to-sd/config"
	"sort"
)

type ByMetricTypeReversed []*v3.TimeSeries

func (ts ByMetricTypeReversed) Len() int {
	return len(ts)
}

func (ts ByMetricTypeReversed) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}

func (ts ByMetricTypeReversed) Less(i, j int) bool {
	return ts[i].Metric.Type > ts[j].Metric.Type
}

var commonConfig = &config.CommonConfig{
	GceConfig: &config.GceConfig{
		Project:       "test-proj",
		Zone:          "us-central1-f",
		Cluster:       "test-cluster",
		Instance:      "kubernetes-master.c.test-proj.internal",
		MetricsPrefix: "container.googleapis.com/master",
	},
	PodConfig: &config.PodConfig{
		NamespaceId: "",
		PodId:       "machine",
	},
	ComponentName: "testcomponent",
}

var metricTypeGauge = dto.MetricType_GAUGE
var metricTypeCounter = dto.MetricType_COUNTER
var metricTypeHistogram = dto.MetricType_HISTOGRAM
var testMetricName = "test_name"
var testMetricHistogram = "test_histogram"
var unrelatedMetric = "unrelated_metric"
var testMetricDescription = "Description 1"
var testMetricHistogramDescription = "Description 2"

var metrics = map[string]*dto.MetricFamily{
	testMetricName: {
		Name: &testMetricName,
		Type: &metricTypeCounter,
		Help: &testMetricDescription,
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					{
						Name:  stringPtr("labelName"),
						Value: stringPtr("labelValue1"),
					},
				},
				Counter: &dto.Counter{Value: floatPtr(42.0)},
			},
			{
				Label: []*dto.LabelPair{
					{
						Name:  stringPtr("labelName"),
						Value: stringPtr("labelValue2"),
					},
				},
				Counter: &dto.Counter{Value: floatPtr(106.0)},
			},
		},
	},
	processStartTimeMetric: {
		Name: stringPtr(processStartTimeMetric),
		Type: &metricTypeGauge,
		Metric: []*dto.Metric{
			{
				Gauge: &dto.Gauge{Value: floatPtr(1234567890.0)},
			},
		},
	},
	unrelatedMetric: {
		Name: &unrelatedMetric,
		Type: &metricTypeGauge,
		Metric: []*dto.Metric{
			{
				Gauge: &dto.Gauge{Value: floatPtr(23.0)},
			},
		},
	},
	testMetricHistogram: {
		Name: &testMetricHistogram,
		Type: &metricTypeHistogram,
		Help: &testMetricHistogramDescription,
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: intPtr(5),
					SampleSum:   floatPtr(13),
					Bucket: []*dto.Bucket{
						{
							CumulativeCount: intPtr(1),
							UpperBound:      floatPtr(1),
						},
						{
							CumulativeCount: intPtr(4),
							UpperBound:      floatPtr(3),
						},
						{
							CumulativeCount: intPtr(4),
							UpperBound:      floatPtr(5),
						},
						{
							CumulativeCount: intPtr(5),
							UpperBound:      floatPtr(math.Inf(1)),
						},
					},
				},
			},
		},
	},
}

var metricDescriptors = map[string]*v3.MetricDescriptor{
	testMetricName: {
		Type:        "container.googleapis.com/master/testcomponent/test_name",
		Description: testMetricDescription,
		MetricKind:  "CUMULATIVE",
		ValueType:   "INT64",
		Labels: []*v3.LabelDescriptor{
			{
				Key: "labelName",
			},
		},
	},
	processStartTimeMetric: {
		Type:       "container.googleapis.com/master/testcomponent/process_start_time_seconds",
		MetricKind: "GAUGE",
		ValueType:  "INT64",
	},
	unrelatedMetric: {
		Type:       "container.googleapis.com/master/testcomponent/unrelated_metric",
		MetricKind: "GAUGE",
		ValueType:  "INT64",
	},
	testMetricHistogram: {
		Type:        "container.googleapis.com/master/testcomponent/test_histogram",
		Description: testMetricHistogramDescription,
		MetricKind:  "CUMULATIVE",
		ValueType:   "DISTRIBUTION",
	},
}

func TestTranslatePrometheusToStackdriver(t *testing.T) {
	epsilon := float64(0.001)
	cache := NewMetricDescriptorCache(nil, commonConfig)

	ts := TranslatePrometheusToStackdriver(commonConfig, []string{testMetricName, testMetricHistogram}, metrics, cache)

	assert.Equal(t, 3, len(ts))
	// TranslatePrometheusToStackdriver uses maps to represent data, so order of output is randomized.
	sort.Sort(ByMetricTypeReversed(ts))

	for i := 0; i <= 1; i++ {
		metric := ts[i]
		assert.Equal(t, "container.googleapis.com/master/testcomponent/test_name", metric.Metric.Type)
		assert.Equal(t, "INT64", metric.ValueType)
		assert.Equal(t, "CUMULATIVE", metric.MetricKind)

		assert.Equal(t, 1, len(metric.Points))
		assert.Equal(t, "2009-02-13T23:31:30Z", metric.Points[0].Interval.StartTime)

		labels := metric.Metric.Labels
		assert.Equal(t, 1, len(labels))

		if labels["labelName"] == "labelValue1" {
			assert.Equal(t, int64(42), *(metric.Points[0].Value.Int64Value))
		} else if labels["labelName"] == "labelValue2" {
			assert.Equal(t, int64(106), *(metric.Points[0].Value.Int64Value))
		} else {
			t.Errorf("Wrong label labelName value %s", labels["labelName"])
		}
	}

	// Histogram
	metric := ts[2]
	assert.Equal(t, "container.googleapis.com/master/testcomponent/test_histogram", metric.Metric.Type)
	assert.Equal(t, "DISTRIBUTION", metric.ValueType)
	assert.Equal(t, "CUMULATIVE", metric.MetricKind)
	assert.Equal(t, 1, len(metric.Points))

	p := metric.Points[0]
	assert.Equal(t, "2009-02-13T23:31:30Z", p.Interval.StartTime)

	dist := p.Value.DistributionValue
	assert.NotNil(t, dist)
	assert.Equal(t, int64(5), dist.Count)
	assert.InEpsilon(t, 2.6, dist.Mean, epsilon)
	assert.InEpsilon(t, 11.25, dist.SumOfSquaredDeviation, epsilon)

	bounds := dist.BucketOptions.ExplicitBuckets.Bounds
	assert.Equal(t, 3, len(bounds))
	assert.InEpsilon(t, 1, bounds[0], epsilon)
	assert.InEpsilon(t, 3, bounds[1], epsilon)
	assert.InEpsilon(t, 5, bounds[2], epsilon)

	counts := dist.BucketCounts
	assert.Equal(t, 4, len(counts))
	assert.Equal(t, int64(1), counts[0])
	assert.Equal(t, int64(3), counts[1])
	assert.Equal(t, int64(0), counts[2])
	assert.Equal(t, int64(1), counts[3])
}

func TestMetricFamilyToMetricDescriptor(t *testing.T) {
	for metricName, metric := range metrics {
		metricDescriptor := MetricFamilyToMetricDescriptor(commonConfig, metric, nil)
		expectedMetricDescriptor := metricDescriptors[metricName]
		assert.Equal(t, metricDescriptor, expectedMetricDescriptor)
	}
}

func floatPtr(val float64) *float64 {
	ptr := val
	return &ptr
}

func intPtr(val uint64) *uint64 {
	ptr := val
	return &ptr
}

func stringPtr(val string) *string {
	ptr := val
	return &ptr
}
