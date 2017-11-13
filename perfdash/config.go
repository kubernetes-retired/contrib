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

// TestDescription contains test name, output file prefix and parser function.
type TestDescription struct {
	Name             string
	OutputFilePrefix string
	Parser           func(data []byte, buildNumber int, testResult *BuildData)
}

// Tests is a map from test label to test description.
type Tests struct {
	Prefix       string
	Descriptions map[string]TestDescription
}

// Jobs is a map from job name to all supported tests in the job.
type Jobs map[string]Tests

// Buckets is a map from bucket url to all supported jobs in the bucket.
type Buckets map[string]Jobs

var (
	// performanceDescriptions contains metrics exported by a --ginko.focus=[Feature:Performance]
	// e2e test
	performanceDescriptions = map[string]TestDescription{
		"DensityResponsiveness": {
			Name:             "density",
			OutputFilePrefix: "APIResponsiveness",
			Parser:           parseResponsivenessData,
		},
		"DensityResources": {
			Name:             "density",
			OutputFilePrefix: "ResourceUsageSummary",
			Parser:           parseResourceUsageData,
		},
		"DensityPodStartup": {
			Name:             "density",
			OutputFilePrefix: "PodStartupLatency",
			Parser:           parseResponsivenessData,
		},
		"DensityTestPhaseTimer": {
			Name:             "density",
			OutputFilePrefix: "TestPhaseTimer",
			Parser:           parseResponsivenessData,
		},
		"LoadResponsiveness": {
			Name:             "load",
			OutputFilePrefix: "APIResponsiveness",
			Parser:           parseResponsivenessData,
		},
		"LoadResources": {
			Name:             "load",
			OutputFilePrefix: "ResourceUsageSummary",
			Parser:           parseResourceUsageData,
		},
		"LoadTestPhaseTimer": {
			Name:             "load",
			OutputFilePrefix: "TestPhaseTimer",
			Parser:           parseResponsivenessData,
		},
	}

	// TestConfig contains all the test PerfDash supports now. Downloader will download and
	// analyze build log from all these Jobs, and parse the data from all these Test.
	// Notice that all the tests should have different name for now.
	// TODO(porridge): automate fetching the list of job list
	TestConfig = Buckets{
		utils.KubekinsBucket: Jobs{
			"ci-kubernetes-e2e-gci-gce-scalability": Tests{
				Prefix:       "gce-100Nodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gci-gce-scalability-stable1": Tests{
				Prefix:       "gce-100Nodes-1.8",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gci-gce-scalability-release-1-7": Tests{
				Prefix:       "gce-100Nodes-1.7-gci",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gce-scalability-release-1-7": Tests{
				Prefix:       "gce-100Nodes-1.7-cvm",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gce-large-performance": Tests{
				Prefix:       "gce-2kNodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gce-scale-performance": Tests{
				Prefix:       "gce-5kNodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-e2e-gke-large-performance": Tests{
				Prefix:       "gke-2kNodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-kubemark-100-gce": Tests{
				Prefix:       "kubemark-100Nodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-kubemark-high-density-100-gce": Tests{
				Prefix:       "kubemark-100Nodes-master-hd",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-kubemark-500-gce": Tests{
				Prefix:       "kubemark-500Nodes-master",
				Descriptions: performanceDescriptions,
			},
			"ci-kubernetes-kubemark-gce-scale": Tests{
				Prefix:       "kubemark-5kNodes-master",
				Descriptions: performanceDescriptions,
			},
		},
	}
)
