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
)

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
