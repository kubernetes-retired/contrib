/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"bufio"
	"fmt"
	"os"

	"k8s.io/contrib/test-utils/utils"
	"k8s.io/kubernetes/pkg/util/sets"
)

// constants to use for downloading data.
const (
	jobName = "kubernetes-e2e-gce-scalability"
	logFile = "build-log.txt"
)

// GoogleGCSDownloader that gets data about Google results from the GCS repository
type GoogleGCSDownloader struct {
	startFrom int
}

// NewGoogleGCSDownloader creates a new GoogleGCSDownloader
func NewGoogleGCSDownloader(startFrom int) *GoogleGCSDownloader {
	return &GoogleGCSDownloader{
		startFrom: startFrom,
	}
}

func (g *GoogleGCSDownloader) getData() (TestToBuildData, sets.String, sets.String, error) {
	fmt.Print("Getting Data from GCS...\n")
	buildLatency := TestToBuildData{}
	resources := sets.NewString()
	methods := sets.NewString()

	buildNumber := g.startFrom
	lastBuildNo, err := utils.GetLastestBuildNumberFromJenkinsGoogleBucket(jobName)
	if err != nil {
		return buildLatency, resources, methods, err
	}
	if buildNumber < lastBuildNo-100 {
		buildNumber = lastBuildNo - 100
	}

	for ; buildNumber <= lastBuildNo; buildNumber++ {
		fmt.Printf("Fetching build %v...\n", buildNumber)
		testDataResponse, err := utils.GetFileFromJenkinsGoogleBucket(jobName, buildNumber, logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while fetching data: %v\n", err)
			continue
		}

		testDataBody := testDataResponse.Body
		defer testDataBody.Close()
		testDataScanner := bufio.NewScanner(testDataBody)

		hist := parseTestOutput(testDataScanner, buildNumber, resources, methods)

		for k, v := range hist {
			if _, ok := buildLatency[k]; !ok {
				buildLatency[k] = make(BuildLatencyData)
			}
			buildLatency[k][fmt.Sprintf("%d", buildNumber)] = v
		}
	}

	return buildLatency, resources, methods, nil
}
