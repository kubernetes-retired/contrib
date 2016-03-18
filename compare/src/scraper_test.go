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
	"bufio"
	"os"
	"reflect"
	"testing"

	k8smetrics "k8s.io/kubernetes/pkg/metrics"
	"k8s.io/kubernetes/test/e2e"

	"github.com/prometheus/common/model"
)

func TestProcessSingleTestLogs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	file, err := os.Open(wd + "/test-data/logs.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	logs, resources, metrics := ProcessSingleTest(scanner, 123)
	if len(resources) != 0 {
		t.Errorf("Received non-empty resources: %v", resources)
	}
	if len(metrics) != 0 {
		t.Errorf("Received non-empty metrics: %v", metrics)
	}
	if len(logs) == 0 {
		t.Error("Didn't receive any logs.")
	} else {
		expectedLogs := &e2e.LogsSizeDataSummary{
			"104.154.36.5:22": map[string]e2e.SingleLogSummary{
				"/var/log/kube-proxy.log": {
					AverageGenerationRate: 700, NumberOfProbes: 2,
				},
				"/var/log/kubelet.log": {
					AverageGenerationRate: 24361, NumberOfProbes: 2,
				},
			},
			"104.197.128.82:22": map[string]e2e.SingleLogSummary{
				"/var/log/kube-proxy.log": {
					AverageGenerationRate: 682, NumberOfProbes: 2,
				},
				"/var/log/kubelet.log": {
					AverageGenerationRate: 22314, NumberOfProbes: 2,
				},
			},
			"104.197.56.13:22": map[string]e2e.SingleLogSummary{
				"/var/log/kubelet.log": {
					AverageGenerationRate: 21990, NumberOfProbes: 2,
				},
				"/var/log/kube-proxy.log": {
					AverageGenerationRate: 682, NumberOfProbes: 2,
				},
			},
			"23.236.62.166:22": map[string]e2e.SingleLogSummary{
				"/var/log/kubelet.log": {
					AverageGenerationRate: 1789, NumberOfProbes: 2,
				},
				"/var/log/kube-addons.log": {
					AverageGenerationRate: 0, NumberOfProbes: 2,
				},
				"/var/log/kube-apiserver.log": {
					AverageGenerationRate: 6610, NumberOfProbes: 2,
				},
				"/var/log/kube-controller-manager.log": {
					AverageGenerationRate: 13314, NumberOfProbes: 2,
				},
				"/var/log/kube-master-addons.log": {
					AverageGenerationRate: 0, NumberOfProbes: 2,
				},
				"/var/log/kube-scheduler.log": {
					AverageGenerationRate: 2646, NumberOfProbes: 2,
				},
			},
		}
		if !reflect.DeepEqual(logs["Sample test"], expectedLogs) {
			t.Errorf("Parsed logs do not match expected value:\nReceived:\n%v\nExpected:\n%v", logs["Sample test"], expectedLogs)
		}
	}
}

func TestProcessSingleTestResources(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	file, err := os.Open(wd + "/test-data/resources.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	logs, resources, metrics := ProcessSingleTest(scanner, 123)
	if len(logs) != 0 {
		t.Errorf("Received non-empty logs: %v", logs)
	}
	if len(metrics) != 0 {
		t.Errorf("Received non-empty metrics: %v", metrics)
	}
	if len(resources) == 0 {
		t.Error("Didn't receive any resources.")
	} else {
		expectedResources := &e2e.ResourceUsageSummary{
			"90": []e2e.SingleContainerSummary{
				{
					Name: "elasticsearch-logging-v1-qvcte/elasticsearch-logging",
					Cpu:  0.060779454406191896,
					Mem:  1199001600,
				},
				{
					Name: "elasticsearch-logging-v1-rlcdl/elasticsearch-logging",
					Cpu:  0.07141353781663302,
					Mem:  1016840192,
				},
				{
					Name: "heapster-v10-8zg3j/heapster",
					Cpu:  0.012815978918275425,
					Mem:  44511232,
				},
				{
					Name: "kube-dns-v10-aenmc/etcd",
					Cpu:  0.006813616250669234,
					Mem:  13012992,
				},
				{
					Name: "kube-dns-v10-aenmc/healthz",
					Cpu:  0.001173291677536088,
					Mem:  1257472,
				},
				{
					Name: "kube-dns-v10-aenmc/kube2sky",
					Cpu:  0.005867475543394822,
					Mem:  7106560,
				},
				{
					Name: "kube-dns-v10-aenmc/skydns",
					Cpu:  0.0012983584381255876,
					Mem:  3805184,
				},
			},
			"99": []e2e.SingleContainerSummary{
				{
					Name: "elasticsearch-logging-v1-qvcte/elasticsearch-logging",
					Cpu:  0.060779454406191896,
					Mem:  1199001600,
				},
				{
					Name: "elasticsearch-logging-v1-rlcdl/elasticsearch-logging",
					Cpu:  0.07141353781663302,
					Mem:  1016840192,
				},
				{
					Name: "heapster-v10-8zg3j/heapster",
					Cpu:  0.012815978918275425,
					Mem:  44511232,
				},
				{
					Name: "kube-dns-v10-aenmc/etcd",
					Cpu:  0.006813616250669234,
					Mem:  13012992,
				},
				{
					Name: "kube-dns-v10-aenmc/healthz",
					Cpu:  0.001173291677536088,
					Mem:  1257472,
				},
				{
					Name: "kube-dns-v10-aenmc/kube2sky",
					Cpu:  0.005867475543394822,
					Mem:  7106560,
				},
				{
					Name: "kube-dns-v10-aenmc/skydns",
					Cpu:  0.0012983584381255876,
					Mem:  3805184,
				},
			},
		}
		if !reflect.DeepEqual(resources["Sample test"], expectedResources) {
			t.Errorf("Parsed resources do not match expected value:\nReceived:\n%v\nExpected:\n%v", resources["Sample test"], expectedResources)
		}
	}
}

func TestProcessSingleTestMetrics(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	file, err := os.Open(wd + "/test-data/metrics.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	logs, resources, metrics := ProcessSingleTest(scanner, 123)
	if len(logs) != 0 {
		t.Errorf("Received non-empty logs: %v", logs)
	}
	if len(resources) != 0 {
		t.Errorf("Received non-empty resources: %v", resources)
	}
	if len(metrics) == 0 {
		t.Error("Didn't receive any metrics.")
	} else {
		expectedMetrics := e2e.MetricsForE2E{
			ApiServerMetrics: k8smetrics.ApiServerMetrics{
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
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
							"code":     "200",
							"resource": "pods",
							"verb":     "GET",
						},
						Value: 8,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
							"code":     "200",
							"resource": "pods",
							"verb":     "LIST",
						},
						Value: 2,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
							"code":     "200",
							"resource": "pods",
							"verb":     "POST",
						},
						Value: 3,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
							"code":     "200",
							"resource": "pods",
							"verb":     "WATCHLIST",
						},
						Value: 2,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "e2e.test/v1.2.0 (linux/amd64) kubernetes/ba8efbd",
							"code":     "200",
							"resource": "replicationcontrollers",
							"verb":     "POST",
						},
						Value: 1,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "heapster/v1.0.3 (linux/amd64) kubernetes/$Format",
							"code":     "200",
							"resource": "namespaces",
							"verb":     "LIST",
						},
						Value: 1,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "heapster/v1.0.3 (linux/amd64) kubernetes/$Format",
							"code":     "200",
							"resource": "namespaces",
							"verb":     "WATCHLIST",
						},
						Value: 1,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "kube-apiserver/v1.2.0 (linux/amd64) kubernetes/e0c876b",
							"code":     "200",
							"resource": "limitranges",
							"verb":     "WATCHLIST",
						},
						Value: 1,
					},
					&model.Sample{
						Metric: model.Metric{
							"__name__": "apiserver_request_count",
							"client":   "kube-apiserver/v1.2.0 (linux/amd64) kubernetes/e0c876b",
							"code":     "200",
							"resource": "resourcequotas",
							"verb":     "WATCHLIST",
						},
						Value: 1,
					},
				},
			},
			ControllerManagerMetrics: k8smetrics.ControllerManagerMetrics{},
			KubeletMetrics:           map[string]k8smetrics.KubeletMetrics{},
			SchedulerMetrics:         k8smetrics.SchedulerMetrics{},
		}
		if !metrics["Sample test"].ApiServerMetrics.Equal(expectedMetrics.ApiServerMetrics) {
			t.Errorf("Parsed APIServer metrics do not match expected value:\nReceived:\n%v\nExpected:\n%v", metrics["Sample test"], expectedMetrics)
		}
		if metrics["Sample test"].ControllerManagerMetrics != nil {
			t.Errorf("Found unexpected ControllerManagerMetrics in the result: %v", metrics["Sample test"].ControllerManagerMetrics)
		}
		if metrics["Sample test"].KubeletMetrics != nil {
			t.Errorf("Found unexpected KubeletMetrics in the result: %v", metrics["Sample test"].KubeletMetrics)
		}
		if metrics["Sample test"].SchedulerMetrics != nil {
			t.Errorf("Found unexpected SchedulerMetrics in the result: %v", metrics["Sample test"].SchedulerMetrics)
		}
	}
}
