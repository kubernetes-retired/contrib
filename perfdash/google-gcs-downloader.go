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
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/contrib/test-utils/utils"
)

// GoogleGCSDownloader that gets data about Google results from the GCS repository
type GoogleGCSDownloader struct {
	Builds               int
	GoogleGCSBucketUtils *utils.Utils
}

// NewGoogleGCSDownloader creates a new GoogleGCSDownloader
func NewGoogleGCSDownloader(builds int) *GoogleGCSDownloader {
	return &GoogleGCSDownloader{
		Builds:               builds,
		GoogleGCSBucketUtils: utils.NewUtils(utils.KubekinsBucket, utils.LogDir),
	}
}

// TODO(random-liu): Only download and update new data each time.
func (g *GoogleGCSDownloader) getData() (TestToBuildData, error) {
	fmt.Print("Getting Data from GCS...\n")
	result := make(TestToBuildData)
	for job, tests := range TestConfig[utils.KubekinsBucket] {
		if tests.Prefix == "" {
			return result, fmt.Errorf("Invalid empty Prefix for job %s", job)
		}
		lastBuildNo, err := g.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
		if err != nil {
			return result, err
		}
		fmt.Printf("Last build no for %v: %v\n", job, lastBuildNo)
		for buildNumber := lastBuildNo; buildNumber > lastBuildNo-g.Builds && buildNumber > 0; buildNumber-- {
			fmt.Printf("Fetching %s build %v...\n", job, buildNumber)
			for testLabel, testDescription := range tests.Descriptions {
				fileStem := fmt.Sprintf("artifacts/%v_%v", testDescription.OutputFilePrefix, testDescription.Name)
				artifacts, err := g.GoogleGCSBucketUtils.ListFilesInBuild(job, buildNumber, fileStem)
				if err != nil || len(artifacts) == 0 {
					fmt.Printf("Error while looking for %s* in build %v: %v\n", fileStem, buildNumber, err)
					continue
				}
				metricsFilename := artifacts[0][strings.LastIndex(artifacts[0], "/")+1:]
				if len(artifacts) > 1 {
					fmt.Printf("WARNING: found multiple %s files with data, reading only one: %s\n", fileStem, metricsFilename)
				}
				testDataResponse, err := g.GoogleGCSBucketUtils.GetFileFromJenkinsGoogleBucket(job, buildNumber, fmt.Sprintf("artifacts/%v", metricsFilename))
				if err != nil {
					panic(err)
				}

				func() {
					testDataBody := testDataResponse.Body
					defer testDataBody.Close()
					data, err := ioutil.ReadAll(testDataBody)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error when reading response Body: %v\n", err)
						return
					}
					testDescription.Parser(data, buildNumber, job, tests.Prefix + "-" + testLabel, result)
				}()
			}
		}
	}
	return result, nil
}
