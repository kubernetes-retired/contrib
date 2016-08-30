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
	"io"

	"k8s.io/contrib/test-utils/utils"
)

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
