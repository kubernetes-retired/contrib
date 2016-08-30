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
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"k8s.io/contrib/test-utils/utils"
)

const (
	latestBuildFile = "latest-build.txt"
	testResultFile  = "build-log.txt"
	kubeletLogFile  = "kubelet.log"
)

// Downloader is the interface that gets a data from a predefined source.
type Downloader interface {
	GetLastestBuildNumber(job string) (int, error)
	GetFile(job string, buildNumber int, logFilePath string) (io.ReadCloser, error)
}

// TODO(random-liu): Only download and update new data each time.
func GetData(d Downloader) (TestToBuildData, error) {
	fmt.Print("Getting Data from test_log...\n")
	result := make(TestToBuildData)
	job := *jenkinsJob
	buildNr := *builds

	lastBuildNo, err := d.GetLastestBuildNumber(job)
	if err != nil {
		return result, err
	}

	fmt.Printf("Last build no: %v\n", lastBuildNo)
	for buildNumber := lastBuildNo; buildNumber > lastBuildNo-buildNr && buildNumber > 0; buildNumber-- {
		fmt.Printf("Fetching build %v...\n", buildNumber)

		file, err := d.GetFile(job, buildNumber, testResultFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while fetching data: %v\n", err)
			return result, err
		}

		parseTestOutput(bufio.NewScanner(file), job, buildNumber, result)
		file.Close()

		if *tracing {
			tracingData := ParseTracing(d, job, buildNumber)
			parseTracingData(bufio.NewScanner(strings.NewReader(tracingData)), job, buildNumber, result)
		}
	}

	return result, nil
}

// LocalDownloader that gets data about Google results from the GCS repository
type LocalDownloader struct {
}

// NewLocalDownloader creates a new LocalDownloader
func NewLocalDownloader() *LocalDownloader {
	return &LocalDownloader{}
}

func (d *LocalDownloader) GetLastestBuildNumber(job string) (int, error) {
	file, err := os.Open(path.Join(*localDataDir, latestBuildFile))
	if err != nil {
		return -1, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()

	i, err := strconv.Atoi(scanner.Text())
	if err != nil {
		log.Fatal(err)
		return -1, err
	}
	return i, nil
}

func (d *LocalDownloader) GetFile(job string, buildNumber int, filePath string) (io.ReadCloser, error) {
	return os.Open(path.Join(*localDataDir, fmt.Sprintf("%d", buildNumber), filePath))
}

// GoogleGCSDownloader that gets data about Google results from the GCS repository
type GoogleGCSDownloader struct {
	GoogleGCSBucketUtils *utils.Utils
}

// NewGoogleGCSDownloader creates a new GoogleGCSDownloader
func NewGoogleGCSDownloader() *GoogleGCSDownloader {
	return &GoogleGCSDownloader{
		GoogleGCSBucketUtils: utils.NewUtils(utils.KubekinsBucket, utils.LogDir),
	}
}

func (d *GoogleGCSDownloader) GetLastestBuildNumber(job string) (int, error) {
	// It returns -1 if the path is not found
	return d.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
}

func (d *GoogleGCSDownloader) GetFile(job string, buildNumber int, filePath string) (io.ReadCloser, error) {
	response, err := d.GoogleGCSBucketUtils.GetFileFromJenkinsGoogleBucket(job, buildNumber, filePath)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
