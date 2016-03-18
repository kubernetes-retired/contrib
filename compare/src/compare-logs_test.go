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
	"reflect"
	"testing"

	"k8s.io/kubernetes/test/e2e"
)

func TestLogCompareSuccess(t *testing.T) {
	left := e2e.LogsSizeDataSummary{
		"104.154.36.5:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 700,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 24361,
				NumberOfProbes:        2,
			},
		},
		"104.197.128.82:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 682,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 22314,
				NumberOfProbes:        2,
			},
		},
		"23.236.62.166:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-apiserver.log": {
				AverageGenerationRate: 6610,
				NumberOfProbes:        2,
			},
			"/var/log/kube-controller-manager.log": {
				AverageGenerationRate: 13314,
				NumberOfProbes:        2,
			},
			"/var/log/kube-master-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-scheduler.log": {
				AverageGenerationRate: 2646,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 1789,
				NumberOfProbes:        2,
			},
		},
	}
	right := e2e.LogsSizeDataSummary{
		"104.154.36.123:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 648,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 25689,
				NumberOfProbes:        2,
			},
		},
		"104.197.128.32:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 712,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 21932,
				NumberOfProbes:        2,
			},
		},
		"23.236.42.55:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-addons.log": {
				AverageGenerationRate: 10,
				NumberOfProbes:        2,
			},
			"/var/log/kube-apiserver.log": {
				AverageGenerationRate: 6615,
				NumberOfProbes:        2,
			},
			"/var/log/kube-controller-manager.log": {
				AverageGenerationRate: 12934,
				NumberOfProbes:        2,
			},
			"/var/log/kube-master-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-scheduler.log": {
				AverageGenerationRate: 2834,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 1992,
				NumberOfProbes:        2,
			},
		},
	}

	if violating := CompareLogGenerationSpeed(&left, &right); violating != nil {
		t.Errorf("Expected compare to return empty list of violating results. Got: %v", violating)
	}
}

func TestLogCompareDifferentSizes(t *testing.T) {
	left := e2e.LogsSizeDataSummary{
		"104.154.36.5:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 700,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 24361,
				NumberOfProbes:        2,
			},
		},
		"23.236.62.166:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-apiserver.log": {
				AverageGenerationRate: 6610,
				NumberOfProbes:        2,
			},
			"/var/log/kube-controller-manager.log": {
				AverageGenerationRate: 13314,
				NumberOfProbes:        2,
			},
			"/var/log/kube-master-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-scheduler.log": {
				AverageGenerationRate: 2646,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 1789,
				NumberOfProbes:        2,
			},
		},
	}
	right := e2e.LogsSizeDataSummary{
		"104.154.36.123:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 648,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 25689,
				NumberOfProbes:        2,
			},
		},
		"104.197.128.32:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 712,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 21932,
				NumberOfProbes:        2,
			},
		},
		"23.236.42.55:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-addons.log": {
				AverageGenerationRate: 10,
				NumberOfProbes:        2,
			},
			"/var/log/kube-apiserver.log": {
				AverageGenerationRate: 6615,
				NumberOfProbes:        2,
			},
			"/var/log/kube-controller-manager.log": {
				AverageGenerationRate: 12934,
				NumberOfProbes:        2,
			},
			"/var/log/kube-master-addons.log": {
				AverageGenerationRate: 0,
				NumberOfProbes:        2,
			},
			"/var/log/kube-scheduler.log": {
				AverageGenerationRate: 2834,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 1992,
				NumberOfProbes:        2,
			},
		},
	}

	if violating := CompareLogGenerationSpeed(&left, &right); violating != nil {
		t.Errorf("Expected compare to return empty list of violating results. Got: %v", violating)
	}
}

func TestLogCompareFailure(t *testing.T) {
	left := e2e.LogsSizeDataSummary{
		"104.154.36.5:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 700,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 24361,
				NumberOfProbes:        2,
			},
		},
	}
	right := e2e.LogsSizeDataSummary{
		"104.197.128.32:22": map[string]e2e.SingleLogSummary{
			"/var/log/kube-proxy.log": {
				AverageGenerationRate: 712,
				NumberOfProbes:        2,
			},
			"/var/log/kubelet.log": {
				AverageGenerationRate: 2193,
				NumberOfProbes:        2,
			},
		},
	}

	expected := ViolatingLogGenerationData{
		"/var/log/kubelet.log": ViolatingLogGenerationPair{
			left: logsDataArray{
				logsDataOnNode{
					node:           "104.154.36.5:22",
					generationRate: 24361,
				},
			},
			right: logsDataArray{
				logsDataOnNode{
					node:           "104.197.128.32:22",
					generationRate: 2193,
				},
			},
		},
	}

	if violating := CompareLogGenerationSpeed(&left, &right); violating == nil {
		t.Errorf("Expected compare to return non empty list of violating results.")
	} else if !reflect.DeepEqual(violating, expected) {
		t.Errorf("Expeted %v got %v", expected, violating)
	}
}
