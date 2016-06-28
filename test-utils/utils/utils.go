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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

const (
	// KubekinsBucket is the name of the kubekins bucket
	KubekinsBucket = "kubernetes-jenkins"
	// LogDir is the directory of kubekins
	LogDir = "logs"
	// LogDir is the directory of the pr builder jenkins
	PRLogDir = "pr-logs"

	successString = "SUCCESS"
)

// Utils is a struct handling all communication with a given bucket
type Utils struct {
	bucket    *Bucket
	directory string
}

// NewUtils returnes new Utils struct for a given bucket name and subdirectory
func NewUtils(bucket, directory string) *Utils {
	return &Utils{
		bucket:    NewBucket(bucket),
		directory: directory,
	}
}

// NewTestUtils returnes new Utils struct for a given url pointing to a file server.
func NewTestUtils(bucket, directory, url string) *Utils {
	return &Utils{
		bucket:    NewTestBucket(bucket, url),
		directory: directory,
	}
}

// GetGCSDirectoryURL returns the url of the bucket directory
func (u *Utils) GetGCSDirectoryURL() string {
	return u.bucket.ExpandPathURL(u.directory).String()
}

// GetPathToJenkinsGoogleBucket returns a GCS path containing the artifacts for a given job and buildNumber.
// This only formats the path. It doesn't include a host or protocol necessary for a full URI.
func (u *Utils) GetPathToJenkinsGoogleBucket(job string, buildNumber int) string {
	return u.bucket.ExpandPathURL(u.directory, job, buildNumber).Path + "/"
}

// GetFileFromJenkinsGoogleBucket reads data from Google project's GCS bucket for the given job and buildNumber.
// Returns a response with file stored under a given (relative) path or an error.
func (u *Utils) GetFileFromJenkinsGoogleBucket(job string, buildNumber int, path string) (*http.Response, error) {
	return u.bucket.ReadFile(u.directory, job, buildNumber, path)
}

// GetLastestBuildNumberFromJenkinsGoogleBucket reads a the number
// of last completed build of the given job from the Google project's GCS bucket .
func (u *Utils) GetLastestBuildNumberFromJenkinsGoogleBucket(job string) (int, error) {
	response, err := u.bucket.ReadFile(u.directory, job, "latest-build.txt")
	if err != nil {
		return -1, err
	}
	body := response.Body
	defer body.Close()
	if response.StatusCode != http.StatusOK {
		glog.Infof("Got a non-success response %v while reading data for %v/latest-build.txt", response.StatusCode, job)
		return -1, err
	}
	scanner := bufio.NewScanner(body)
	scanner.Scan()
	var lastBuildNo int
	fmt.Sscanf(scanner.Text(), "%d", &lastBuildNo)
	return lastBuildNo, nil
}

// StartedFile is a type in which we store test starting informatio in GCS as started.json
type StartedFile struct {
	Version     string `json:"version"`
	Timestamp   uint64 `json:"timestamp"`
	JenkinsNode string `json:"jenkins-node"`
}

// CheckStartedStatus reads the started.json file for a given job and build number.
// It returns true if the result stored there is success, and false otherwise.
func (u *Utils) CheckStartedStatus(job string, buildNumber int) (*StartedFile, error) {
	response, err := u.bucket.ReadFile(u.directory, job, buildNumber, "started.json")
	if err != nil {
		glog.Errorf("Error while getting data for %v/%v/%v: %v", job, buildNumber, "started.json", err)
		return nil, err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		glog.Errorf("Got a non-success response %v while reading data for %v/%v/%v", response.StatusCode, job, buildNumber, "started.json")
		return nil, err
	}
	result := &StartedFile{}
	err = json.NewDecoder(response.Body).Decode(result)
	if err != nil {
		glog.Errorf("Failed to read or unmarshal %v/%v/%v: %v", job, buildNumber, "started.json", err)
		return nil, err
	}
	return result, nil
}

// FinishedFile is a type in which we store test result in GCS as finished.json
type FinishedFile struct {
	Result    string `json:"result"`
	Timestamp uint64 `json:"timestamp"`
}

// CheckFinishedStatus reads the finished.json file for a given job and build number.
// It returns true if the result stored there is success, and false otherwise.
func (u *Utils) CheckFinishedStatus(job string, buildNumber int) (bool, error) {
	response, err := u.bucket.ReadFile(u.directory, job, buildNumber, "finished.json")
	if err != nil {
		glog.Errorf("Error while getting data for %v/%v/%v: %v", job, buildNumber, "finished.json", err)
		return false, err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		glog.Errorf("Got a non-success response %v while reading data for %v/%v/%v", response.StatusCode, job, buildNumber, "finished.json")
		return false, fmt.Errorf("got status code %v", response.StatusCode)
	}
	result := FinishedFile{}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		glog.Errorf("Failed to read the response for %v/%v/%v: %v", job, buildNumber, "finished.json", err)
		return false, err
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		glog.Errorf("Failed to unmarshal %v: %v", string(body), err)
		return false, err
	}
	return result.Result == successString, nil
}

// ListFilesInBuild takes build info and list all file names with matching prefix
// The returned file name included the complete path from bucket root
func (u *Utils) ListFilesInBuild(job string, buildNumber int, prefix string) ([]string, error) {
	if u.needsDeref(job) {
		dir, err := u.deref(job, buildNumber)
		if err != nil {
			return nil, fmt.Errorf("Couldn't deref %v/%v: %v", job, buildNumber, err)
		}
		return u.bucket.List(dir, prefix)
	}

	return u.bucket.List(u.directory, job, buildNumber, prefix)
}

// ListFilesWithPrefix returns all files with matching prefix in the bucket
// The returned file name included the complete path from bucket root
func (u *Utils) ListFilesWithPrefix(prefix string) ([]string, error) {
	return u.bucket.List(prefix)
}
