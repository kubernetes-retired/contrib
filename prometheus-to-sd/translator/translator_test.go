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
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"k8s.io/contrib/prometheus-to-sd/config"
)

func TestTranslatePrometheusToStackdriver(t *testing.T) {
	config := &config.GceConfig{
		Project:  "test-proj",
		Zone:     "us-central1-f",
		Cluster:  "test-cluster",
		Instance: "kubernetes-master.c.test-proj.internal",
		Prefix:   "container.googleapis.com/master",
	}

	metricTypeGauge := dto.MetricType_GAUGE
	metricTypeCounter := dto.MetricType_COUNTER
	testMetricName := "test_name"
	unrelatedMetric := "unrelated_metric"

	metrics := map[string]*dto.MetricFamily{
		testMetricName: {
			Name: &testMetricName,
			Type: &metricTypeCounter,
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
	}

	ts := TranslatePrometheusToStackdriver(config, "testcomponent", metrics, testMetricName)

	assert.Equal(t, 2, len(ts))

	for _, metric := range ts {
		assert.Equal(t, "container.googleapis.com/master/testcomponent/test_name", metric.Metric.Type)
		assert.Equal(t, "INT64", metric.ValueType)
		assert.Equal(t, "CUMULATIVE", metric.MetricKind)

		assert.Equal(t, 1, len(metric.Points))
		assert.Equal(t, "2009-02-14T00:31:30+01:00", metric.Points[0].Interval.StartTime)

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
}

func floatPtr(val float64) *float64 {
	ptr := val
	return &ptr
}

func stringPtr(val string) *string {
	ptr := val
	return &ptr
}
