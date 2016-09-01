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
	"math"
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

var (
	grabbedLastBuild int
	// allTestData stores all parsed perf and time series data in memeory.
	allTestData = TestToBuildData{}
)

// Downloader is the interface that gets a data from a predefined source.
type Downloader interface {
	GetLastestBuildNumber(job string) (int, error)
	GetFile(job string, buildNumber int, logFilePath string) (io.ReadCloser, error)
}

// GetData fetch as much data as possible and result the result.
func GetData(d Downloader, allTestData TestToBuildData) error {
	fmt.Printf("Getting Data from %s...\n", *datasource)
	job := *jenkinsJob
	buildNr := *builds

	lastBuildNo, err := d.GetLastestBuildNumber(job)
	if err != nil {
		return err
	}

	fmt.Printf("Last build no: %v\n", lastBuildNo)

	endBuild := lastBuildNo
	startBuild := int(math.Max(math.Max(float64(lastBuildNo-buildNr), 0), float64(grabbedLastBuild))) + 1

	for buildNumber := startBuild; buildNumber <= endBuild; buildNumber++ {
		fmt.Printf("Fetching build %v...\n", buildNumber)

		file, err := d.GetFile(job, buildNumber, testResultFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while fetching data: %v\n", err)
			return err
		}

		testTime := TestTime{}
		parseTestOutput(bufio.NewScanner(file), job, buildNumber, allTestData, testTime)
		file.Close()

		// TODO(coufon): currently we run one test per node. we must check multi-test per node
		// to make sure test separation by 'end time of test' works.

		// It contains test end time information, used to extract event logs
		//fmt.Printf("%#v\n", testTime.Sort())

		if *tracing {
			tracingData := ParseKubeletLog(d, job, buildNumber, testTime)
			parseTracingData(bufio.NewScanner(strings.NewReader(tracingData)), job, buildNumber, allTestData)
		}
	}
	grabbedLastBuild = lastBuildNo
	return nil
}

// LocalDownloader that gets data about Google results from the GCS repository
type LocalDownloader struct {
}

// NewLocalDownloader creates a new LocalDownloader
func NewLocalDownloader() *LocalDownloader {
	return &LocalDownloader{}
}

// GetLastestBuildNumber returns the latest build number.
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

// GetFile returns readcloser of the desired file.
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

// GetLastestBuildNumber returns the latest build number.
func (d *GoogleGCSDownloader) GetLastestBuildNumber(job string) (int, error) {
	// It returns -1 if the path is not found
	return d.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
}

// GetFile returns readcloser of the desired file.
func (d *GoogleGCSDownloader) GetFile(job string, buildNumber int, filePath string) (io.ReadCloser, error) {
	response, err := d.GoogleGCSBucketUtils.GetFileFromJenkinsGoogleBucket(job, buildNumber, filePath)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
