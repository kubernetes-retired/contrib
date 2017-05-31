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

package main

import (
	"k8s.io/contrib/test-utils/utils"
)

// To add new e2e test support, you need to:
//   1) Transform e2e performance test result into *PerfData* in k8s/kubernetes/test/e2e/perftype,
//   and print the PerfData in e2e test log.
//   2) Add corresponding bucket, job and test into *TestConfig*.

// Tests is a map from test name to test output file prefix.
type Tests map[string]string

// Jobs is a map from job name to all supported tests in the job.
type Jobs map[string]Tests

// Buckets is a map from bucket url to all supported jobs in the bucket.
type Buckets map[string]Jobs

var (
	// TestConfig contains all the test PerfDash supports now. Downloader will download and
	// analyze build log from all these Jobs, and parse the data from all these Test.
	// Notice that all the tests should have different name for now.
	TestConfig = Buckets{
		utils.KubekinsBucket: Jobs{
			"ci-kubernetes-e2e-gci-gce-scalability": Tests{
				"Density": "APIResponsiveness",
				"Load":    "APIResponsiveness",
			},
			"ci-kubernetes-e2e-gci-gce-serial": Tests{
				"Kubelet Perf 35":  "TBD",
				"Kubelet Perf 100": "TBD",
			},
		},
	}

	// TestNameSeparator is the prefix of time name.
	TestNameSeparator = "[It] "
)
