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
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"k8s.io/contrib/test-utils/utils"
)

const (
	latestBuildFile = "latest-build.txt"
)

// Downloader is the interface that connects to a data source.
type Downloader interface {
	GetLastestBuildNumber(job string) (int, error)
	ListFilesInBuild(job string, build int, prefix string) ([]string, error)
	GetFile(job string, buildNumber int, logFilePath string) (io.ReadCloser, error)
}

// LocalDownloader gets test data from local files.
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

// ListFilesInBuild returns the contents of the files with the specified prefix
// for the test job at the given buildNumber.
func (d *LocalDownloader) ListFilesInBuild(job string, buildNumber int, prefix string) ([]string, error) {
	prefixDir, prefixFile := path.Split(prefix)
	filesInDir, err := ioutil.ReadDir(path.Join(*localDataDir, fmt.Sprintf("%d", buildNumber), prefixDir))
	if err != nil {
		return nil, err
	}
	filesInBuild := []string{}
	for _, file := range filesInDir {
		if strings.HasPrefix(file.Name(), prefixFile) {
			filesInBuild = append(filesInBuild, path.Join(prefixDir, file.Name()))
		}
	}
	return filesInBuild, nil
}

// GetFile returns readcloser of the desired file.
func (d *LocalDownloader) GetFile(job string, buildNumber int, filePath string) (io.ReadCloser, error) {
	return os.Open(path.Join(*localDataDir, fmt.Sprintf("%d", buildNumber), filePath))
}

// GoogleGCSDownloader gets test data from Google Cloud Storage.
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

// ListFilesInBuild returns the contents of the files with the specified prefix
// for the test job at the given buildNumber.
func (d *GoogleGCSDownloader) ListFilesInBuild(job string, buildNumber int, prefix string) ([]string, error) {
	return d.GoogleGCSBucketUtils.ListFilesInBuild(job, buildNumber, prefix)
}

// GetFile returns readcloser of the desired file.
func (d *GoogleGCSDownloader) GetFile(job string, buildNumber int, filePath string) (io.ReadCloser, error) {
	response, err := d.GoogleGCSBucketUtils.GetFileFromJenkinsGoogleBucket(job, buildNumber, filePath)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
