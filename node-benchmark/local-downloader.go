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
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
)

const (
	latestBuildFile = "latest-build.txt"
	tracingParser   = "parse_kubelet_log.py"
)

// LocalDownloader that gets data about Google results from the GCS repository
type LocalDownloader struct {
	Builds int
}

// NewLocalDownloader creates a new LocalDownloader
func NewLocalDownloader(builds int) *LocalDownloader {
	return &LocalDownloader{
		Builds: builds,
	}
}

// TODO(random-liu): Only download and update new data each time.
func (g *LocalDownloader) getData() (TestToBuildData, error) {
	fmt.Print("Getting Data from test_log...\n")
	result := make(TestToBuildData)
	dataDir := *localDataDir

	lastBuildNo := getLastestBuildNumber(dataDir)

	for buildNumber := lastBuildNo; buildNumber > lastBuildNo-g.Builds && buildNumber > 0; buildNumber-- {
		fmt.Printf("Fetching build %v...\n", buildNumber)

		file, err := os.Open(path.Join(dataDir, fmt.Sprintf("%d", buildNumber), "build-log.txt"))
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		testDataScanner := bufio.NewScanner(file)
		parseTestOutput(testDataScanner,
			"kubernetes-e2e-node-benchmark",
			buildNumber,
			result)

		file, err = os.Open(path.Join(dataDir, fmt.Sprintf("%d", buildNumber), "tracing.log"))
		if os.IsNotExist(err) {
			// Tracing data have not been parsed yet
			cmd := exec.Command("/usr/bin/python", tracingParser, dataDir, fmt.Sprintf("%d", buildNumber))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err = cmd.Run(); err != nil {
				log.Fatal(err)
			}
			file, err = os.Open(path.Join(dataDir, fmt.Sprintf("%d", buildNumber), "tracing.log"))
		}
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		testDataScanner = bufio.NewScanner(file)
		parseTracingData(testDataScanner,
			"kubernetes-e2e-node-benchmark",
			buildNumber,
			result)
	}

	return result, nil
}

func getLastestBuildNumber(dataDir string) int {
	file, err := os.Open(path.Join(dataDir, latestBuildFile))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()

	i, err := strconv.Atoi(scanner.Text())
	if err != nil {
		log.Fatal(err)
	}
	return i
}
