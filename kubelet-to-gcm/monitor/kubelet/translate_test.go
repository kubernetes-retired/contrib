/*
Copyright 2016 The Kubernetes Authors.

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

package kubelet

import (
	"encoding/json"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/stats"
)

const (
	summaryJSON = `{
    "node": {
        "cpu": {
            "time": "2016-06-09T23:23:43Z",
            "usageCoreNanoSeconds": 10000000000,
            "usageNanoCores": 1000000000
        },
        "fs": {
            "availableBytes": 6000,
            "capacityBytes": 10000,
            "usedBytes": 4000
        },
        "memory": {
            "majorPageFaults": 6,
            "pageFaults": 10,
            "rssBytes": 2900,
            "time": "2016-06-09T23:23:43Z",
            "usageBytes": 2800,
            "workingSetBytes": 2700
        },
        "network": {
            "rxBytes": 1000,
            "rxErrors": 0,
            "time": "2016-06-09T23:23:43Z",
            "txBytes": 5000,
            "txErrors": 0
        },
        "nodeName": "gke-365122390874-ce73d81691de44a798a4",
        "startTime": "2016-06-08T00:25:37Z",
        "systemContainers": [
            {
                "cpu": {
                    "time": "2016-06-09T23:23:45Z",
                    "usageCoreNanoSeconds": 10000000000,
                    "usageNanoCores": 1000000000
                },
                "logs": {
                    "availableBytes": 5000,
                    "capacityBytes": 10000
                },
                "memory": {
                    "majorPageFaults": 5,
                    "pageFaults": 10,
                    "rssBytes": 2900,
                    "time": "2016-06-09T23:23:45Z",
                    "usageBytes": 2800,
                    "workingSetBytes": 2700
                },
                "name": "misc",
                "rootfs": {
                    "availableBytes": 6000,
                    "capacityBytes": 10000
                },
                "startTime": "2016-06-08T00:26:41Z",
                "userDefinedMetrics": null
            }
        ]
    },
    "pods": [
        {
            "containers": [
                {
                    "cpu": {
                        "time": "2016-06-09T23:23:51Z",
                        "usageCoreNanoSeconds": 10000000000,
                        "usageNanoCores": 1000000000
                    },
                    "logs": {
                        "availableBytes": 5000,
                        "capacityBytes": 8000,
                        "usedBytes": 3000
                    },
                    "memory": {
                        "majorPageFaults": 6,
                        "pageFaults": 10,
                        "rssBytes": 2900,
                        "time": "2016-06-09T23:23:51Z",
                        "usageBytes": 2800,
                        "workingSetBytes": 2700
                    },
                    "name": "test-container",
                    "rootfs": {
                        "availableBytes": 6000,
                        "capacityBytes": 10000,
                        "usedBytes": 4000
                    },
                    "startTime": "2016-06-08T00:27:48Z",
                    "userDefinedMetrics": null
                },
                {
                    "cpu": {
                        "time": "2016-06-09T23:23:50Z",
                        "usageCoreNanoSeconds": 1127596874,
                        "usageNanoCores": 0
                    },
                    "logs": {
                        "availableBytes": 6214086656,
                        "capacityBytes": 10432602112,
                        "usedBytes": 16384
                    },
                    "memory": {
                        "majorPageFaults": 0,
                        "pageFaults": 21866,
                        "rssBytes": 0,
                        "time": "2016-06-09T23:23:50Z",
                        "usageBytes": 192512,
                        "workingSetBytes": 131072
                    },
                    "name": "fluentd-cloud-logging",
                    "rootfs": {
                        "availableBytes": 6214086656,
                        "capacityBytes": 10432602112,
                        "usedBytes": 28672
                    },
                    "startTime": "2016-06-08T00:27:19Z",
                    "userDefinedMetrics": null
                }
            ],
            "network": {
                "rxBytes": 538477070,
                "rxErrors": 0,
                "time": "2016-06-09T23:23:43Z",
                "txBytes": 2969251391,
                "txErrors": 0
            },
            "podRef": {
                "name": "test-pod",
                "namespace": "kube-system",
                "uid": "e336ead99236b6eac0ce68e5336c86a0"
            },
            "startTime": "2016-06-08T00:27:47Z"
        }
    ]
}`
)

// TestTranslator
func TestTranslator(t *testing.T) {
	testCases := []struct {
		Summary, Zone, Project, Cluster, InstanceID string
		Resolution                                  time.Duration
		ExpectedTSCount                             int
	}{
		{
			Zone:            "us-central1-f",
			Project:         "test-project",
			Cluster:         "unit-test-clus",
			InstanceID:      "this-instance",
			Resolution:      time.Second * time.Duration(10),
			Summary:         summaryJSON,
			ExpectedTSCount: 28,
		},
	}

	for i, tc := range testCases {
		summary := &stats.Summary{}
		if err := json.Unmarshal([]byte(tc.Summary), summary); err != nil {
			t.Errorf("Failed to unmarshal test case %d with data %s, err: %v", i, tc.Summary, err)
		}

		translator := NewTranslator(tc.Zone, tc.Project, tc.Cluster, tc.InstanceID, tc.Resolution)
		tsReq, err := translator.Translate(summary)
		if err != nil {
			t.Errorf("Failed to translate to GCM in test case %d. Summary: %v, Err: %s", i, tc.Summary, err)
		}

		if tc.ExpectedTSCount != len(tsReq.TimeSeries) {
			t.Errorf("Expected %d TimeSeries, got %d", tc.ExpectedTSCount, len(tsReq.TimeSeries))
		}
	}
}
