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

package utils

import (
	"bufio"
	"fmt"
	"net/http"
)

const (
	urlPrefix = "https://storage.googleapis.com/kubernetes-jenkins/logs"
)

// GetFileFromJenkinsGoogleBucket reads data from Google project's GCS bucket for the given job and buildNumber.
// Returns a response with file stored under a given (relative) path or an error.
func GetFileFromJenkinsGoogleBucket(job string, buildNumber int, path string) (*http.Response, error) {
	response, err := http.Get(fmt.Sprintf("%v/%v/%v/%v", urlPrefix, job, buildNumber, path))
	if err != nil {
		return nil, err
	}
	return response, nil
}

// GetLastestBuildNumberFromJenkinsGoogleBucket reads a the number
// of last completed build of the given job from the Google project's GCS bucket .
func GetLastestBuildNumberFromJenkinsGoogleBucket(job string) (int, error) {
	response, err := http.Get(fmt.Sprintf("%v/%v/latest-build.txt", urlPrefix, job))
	if err != nil {
		return -1, err
	}
	body := response.Body
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Scan()
	var lastBuildNo int
	fmt.Sscanf(scanner.Text(), "%d", &lastBuildNo)
	return lastBuildNo, nil
}
