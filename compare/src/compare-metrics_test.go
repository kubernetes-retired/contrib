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

package src

import (
	"math"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/metrics"
	"k8s.io/kubernetes/test/e2e"

	"github.com/prometheus/common/model"
)

func TestMetricCompareSuccess(t *testing.T) {
	left := &e2e.MetricsForE2E{
		ApiServerMetrics: metrics.ApiServerMetrics{
			"apiserver_request_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "LIST",
					},
					Value: 2,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "WATCHLIST",
					},
					Value: 1,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "nodes",
						"verb":     "LIST",
					},
					Value: 4,
				},
			},
		},
		ControllerManagerMetrics: metrics.ControllerManagerMetrics{
			"etcd_helper_cache_entry_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_entry_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_hit_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_hit_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_miss_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_miss_count",
					},
					Value: 0,
				},
			},
			"etcd_request_cache_add_latencies_summary": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.5",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.9",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.99",
					},
					Value: model.SampleValue(math.NaN()),
				},
			},
		},
		KubeletMetrics: map[string]metrics.KubeletMetrics{
			"e2e-test-master": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.000158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.002003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.002483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.002912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.012803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 119,
					},
				},
			},
			"e2e-test-minion": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.001158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.012003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.012483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.012912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.022803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 219,
					},
				},
			},
		},
		SchedulerMetrics: metrics.SchedulerMetrics{
			"get_token_count":      model.Samples{},
			"get_token_fail_count": model.Samples{},
			"go_gc_duration_seconds": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0",
					},
					Value: 0.00014963700000000002,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.25",
					},
					Value: 0.0005699380000000001,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.5",
					},
					Value: 0.001986198,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.75",
					},
					Value: 0.003013346,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "1",
					},
					Value: 0.007081308000000001,
				},
			},
		},
	}
	right := &e2e.MetricsForE2E{
		ApiServerMetrics: metrics.ApiServerMetrics{
			"apiserver_request_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "LIST",
					},
					Value: 3,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "WATCHLIST",
					},
					Value: 1,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "nodes",
						"verb":     "LIST",
					},
					Value: 3,
				},
			},
		},
		ControllerManagerMetrics: metrics.ControllerManagerMetrics{
			"etcd_helper_cache_entry_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_entry_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_hit_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_hit_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_miss_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_miss_count",
					},
					Value: 0,
				},
			},
			"etcd_request_cache_add_latencies_summary": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.5",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.9",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.99",
					},
					Value: model.SampleValue(math.NaN()),
				},
			},
		},
		KubeletMetrics: map[string]metrics.KubeletMetrics{
			"e2e-test-master": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.000158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.002003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.002483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.002912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.012803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 119,
					},
				},
			},
			"e2e-test-minion": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.001158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.012003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.012483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.012912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.022803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 219,
					},
				},
			},
		},
		SchedulerMetrics: metrics.SchedulerMetrics{
			"get_token_count":      model.Samples{},
			"get_token_fail_count": model.Samples{},
			"go_gc_duration_seconds": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0",
					},
					Value: 0.00014963700000000002,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.25",
					},
					Value: 0.0005699380000000001,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.5",
					},
					Value: 0.001986198,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.75",
					},
					Value: 0.003013346,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "1",
					},
					Value: 0.007081308000000001,
				},
			},
		},
	}
	if violated := CompareMetrics(left, right); len(violated) != 0 {
		t.Errorf("Expected compare to return empty list, got %v.", violated)
	}
}

func TestMetricCompareFailure(t *testing.T) {
	left := &e2e.MetricsForE2E{
		ApiServerMetrics: metrics.ApiServerMetrics{
			"apiserver_request_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "LIST",
					},
					Value: 5,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "WATCHLIST",
					},
					Value: 1,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "nodes",
						"verb":     "LIST",
					},
					Value: 4,
				},
			},
		},
		ControllerManagerMetrics: metrics.ControllerManagerMetrics{
			"etcd_helper_cache_entry_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_entry_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_hit_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_hit_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_miss_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_miss_count",
					},
					Value: 0,
				},
			},
			"etcd_request_cache_add_latencies_summary": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.5",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.9",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.99",
					},
					Value: model.SampleValue(math.NaN()),
				},
			},
		},
		KubeletMetrics: map[string]metrics.KubeletMetrics{
			"e2e-test-master": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.000158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.002003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.002483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.002912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.012803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 119,
					},
				},
			},
			"e2e-test-minion": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.001158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.013003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.012483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.012912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.032803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 219,
					},
				},
			},
		},
		SchedulerMetrics: metrics.SchedulerMetrics{
			"get_token_count":      model.Samples{},
			"get_token_fail_count": model.Samples{},
			"go_gc_duration_seconds": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0",
					},
					Value: 0.00014963700000000002,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.25",
					},
					Value: 0.0005699380000000001,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.5",
					},
					Value: 0.001986198,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.75",
					},
					Value: 0.003013446,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "1",
					},
					Value: 0.004081308000000001,
				},
			},
		},
	}
	right := &e2e.MetricsForE2E{
		ApiServerMetrics: metrics.ApiServerMetrics{
			"apiserver_request_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "LIST",
					},
					Value: 3,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "events",
						"verb":     "WATCHLIST",
					},
					Value: 10,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "apiserver_request_count",
						"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
						"code":     "200",
						"resource": "nodes",
						"verb":     "LIST",
					},
					Value: 3,
				},
			},
		},
		ControllerManagerMetrics: metrics.ControllerManagerMetrics{
			"etcd_helper_cache_entry_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_entry_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_hit_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_hit_count",
					},
					Value: 0,
				},
			},
			"etcd_helper_cache_miss_count": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_helper_cache_miss_count",
					},
					Value: 0,
				},
			},
			"etcd_request_cache_add_latencies_summary": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.5",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.9",
					},
					Value: model.SampleValue(math.NaN()),
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "etcd_request_cache_add_latencies_summary",
						"quantile": "0.99",
					},
					Value: model.SampleValue(math.NaN()),
				},
			},
		},
		KubeletMetrics: map[string]metrics.KubeletMetrics{
			"e2e-test-master": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.000158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.002003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.012483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.102912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.012803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 119,
					},
				},
			},
			"e2e-test-minion": {
				"go_gc_duration_seconds": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0",
						},
						Value: 0.001158288,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.25",
						},
						Value: 0.012003092,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.5",
						},
						Value: 0.012483379,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "0.75",
						},
						Value: 0.012912684,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_gc_duration_seconds",
							"quantile": "1",
						},
						Value: 0.022803119,
					},
				},
				"go_goroutines": model.Samples{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "go_goroutines",
						},
						Value: 519,
					},
				},
			},
		},
		SchedulerMetrics: metrics.SchedulerMetrics{
			"get_token_count":      model.Samples{},
			"get_token_fail_count": model.Samples{},
			"go_gc_duration_seconds": model.Samples{
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0",
					},
					Value: 0.00014963700000000002,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.25",
					},
					Value: 0.0005699380000000001,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.5",
					},
					Value: 0.001986198,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "0.75",
					},
					Value: 0.103013346,
				},
				&model.Sample{
					Metric: model.Metric{
						"__name__": "go_gc_duration_seconds",
						"quantile": "1",
					},
					Value: 0.007081308000000001,
				},
			},
		},
	}
	expected := ViolatingMetricsArr{
		"apiserver_request_count": []ViolatingMetric{
			{
				labels:    "client=e2e.test, code=200, resource=events, verb=LIST",
				component: "ApiServer",
				left:      5,
				right:     3,
			},
			{
				labels:    "client=e2e.test, code=200, resource=events, verb=WATCHLIST",
				component: "ApiServer",
				left:      1,
				right:     10,
			},
		},
		"go_goroutines": []ViolatingMetric{
			{
				labels:    "",
				component: "e2e-test-master#e2e-test-master#MIN",
				left:      119,
				right:     119,
			},
			{
				labels:    "",
				component: "e2e-test-minion#e2e-test-minion#MAX",
				left:      219,
				right:     519,
			},
		},
	}

	violated := CompareMetrics(left, right)
	// Easiest way to ignore order in slices during comparison is to create maps from them...
	violatedMap := make(map[string]map[ViolatingMetric]struct{})
	expectedMap := make(map[string]map[ViolatingMetric]struct{})
	for k, v := range violated {
		for _, metric := range v {
			if _, ok := violatedMap[k]; !ok {
				violatedMap[k] = make(map[ViolatingMetric]struct{})
			}
			violatedMap[k][metric] = struct{}{}
		}
	}
	for k, v := range expected {
		for _, metric := range v {
			if _, ok := expectedMap[k]; !ok {
				expectedMap[k] = make(map[ViolatingMetric]struct{})
			}
			expectedMap[k][metric] = struct{}{}
		}
	}
	if !reflect.DeepEqual(expectedMap, violatedMap) {
		t.Errorf("Expected compare to return %v list, got %v.", expected, violated)
	}
}
