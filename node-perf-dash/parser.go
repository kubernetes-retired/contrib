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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/test/e2e/perftype"
	nodeperftype "k8s.io/kubernetes/test/e2e_node/perftype"
)

// supportedMetricVersion is the metric version supported in node-perf-dash.
// node-perf-dash will only parse the metrics with this exact version -- any
// older or newer versions will be ignored.
const supportedMetricVersion = "v2"

var (
	// buildFIFOs is a map from (job, test, node) to a list of build
	// numbers. It is used to find and the oldest build for a
	// (job, test, node).
	buildFIFOs = map[string][]string{}

	// allGrabbedLastBuild stores the last build grabbed for each job.
	allGrabbedLastBuild = map[string]int{}

	// nodeNameCache stores formatted node names, looked up by host name
	// (the machine which runs the test).
	nodeNameCache = map[string]string{}
)

// Parse fetches data from the source and populates allTestData and testInfo
// for the given test job.
func Parse(allTestData map[string]TestToBuildData, testInfo *TestInfo, job string, source Downloader) error {
	fmt.Printf("Getting Data from %s... (Job: %s)\n", *datasource, job)

	grabbedLastBuild := allGrabbedLastBuild[job]

	lastBuildNumber, err := source.GetLastestBuildNumber(job)
	if err != nil {
		return fmt.Errorf("failed to get the lastest build number for job %q", job)
	}
	fmt.Printf("Last build no: %v (Job: %s)\n", lastBuildNumber, job)

	startBuildNumber := int(math.Max(math.Max(float64(lastBuildNumber-*builds), 0), float64(grabbedLastBuild))) + 1
	for buildNumber := lastBuildNumber; buildNumber >= startBuildNumber; buildNumber-- {
		fmt.Printf("Fetching build %v... (Job: %s)\n", buildNumber, job)
		if err := populateDataForOneBuild(allTestData[job], testInfo, job, buildNumber, source); err != nil {
			return err
		}
	}

	allGrabbedLastBuild[job] = lastBuildNumber
	return nil
}

// getDataFromFiles returns the contents of the files having the prefix for the
// test job at the given build. The files will be fetched using the specified
// source.
//
// For example, getDataFromFiles("ci-kubernetes-node-kubelet-benchmark", "1234", "peformance", source)
// returns the contents of all files with the prefix
// "gs://kubernetes-jenkins/logs/ci-kubernetes-node-kubelet-benchmark/1234/artifacts/performance-*".
func getDataFromFiles(job, build, prefix string, source Downloader) ([][]byte, error) {
	buildNumber, err := strconv.Atoi(build)
	if err != nil {
		return nil, fmt.Errorf("failed to convert build number %q to an int: %v", build, err)
	}
	prefix = "artifacts/" + prefix + "-"
	filenames, err := source.ListFilesInBuild(job, buildNumber, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list files with prefix %q: %v", prefix, err)
	}
	var contents [][]byte
	for _, filename := range filenames {
		filename := "artifacts/" + filename[strings.LastIndex(filename, "/")+1:]
		body, err := source.GetFile(job, buildNumber, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to get %q: %v", filename, err)
		}
		defer body.Close()
		data, err := ioutil.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read %q: %v", filename, err)
		}
		contents = append(contents, data)
	}
	return contents, nil
}

// populateMetadata populates the test description in testInfo and the test end
// timestamp in testTime using the information in the given labels.
func populateMetadata(testInfo *TestInfo, testTime *TestTime, labels map[string]string) error {
	test := labels["test"]

	// Populate testInfo with the test description.
	testInfo.Info[test] = labels["desc"]

	// Populate testTime with the test end timestamp.
	t, err := strconv.ParseInt(labels["timestamp"], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert timestamp %q to an int64: %v", labels["timestamp"], err)
	}
	timestamp := time.Unix(t, 0).UTC().Format(testLogTimeFormat)
	testTime.Add(test, labels["node"], timestamp)

	return nil
}

// removeStaledBuilds ensures that the testData only contains data for --builds
// number of builds by removing staled build data.
func removeStaledBuilds(testData TestToBuildData, job, test, node, build string) {
	key := job + "_" + test + "_" + node
	count := len(buildFIFOs[key])
	if count == 0 || buildFIFOs[key][count-1] != build {
		// A new build comes.
		buildFIFOs[key] = append(buildFIFOs[key], build)
		count++
	}
	if count > *builds {
		delete(testData[test].Data[node], buildFIFOs[key][0])
		buildFIFOs[key] = buildFIFOs[key][1:]
	}
}

// TODO(yguo0905): Consider extracting the common logic in
// populatePerformanceData() and populateTimeSeriesData() into a function.

// populatePerformanceData populates the perf data in testData and the metadata
// in testInfo and testTime for the given test job at the given build using the
// data fetched from source.
func populatePerformanceData(testData TestToBuildData, testInfo *TestInfo, testTime *TestTime, job, build string, source Downloader) error {
	contents, err := getDataFromFiles(job, build, "performance", source)
	if err != nil {
		return err
	}
	for _, data := range contents {
		// Decode the data into obj.
		var obj perftype.PerfData
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to parse performance data: %v\ndata=\n%v", err, string(data))
		}

		// Ignore the metrics with unsupported versions.
		if obj.Version != supportedMetricVersion {
			continue
		}

		// Populate the metadata (testInfo and testTime) with the
		// version and labels in the perf data.
		if err := populateMetadata(testInfo, testTime, obj.Labels); err != nil {
			return err
		}

		// Populate the result (testData) with the perf data.
		node := formatNodeName(obj.Labels, job)
		test := obj.Labels["test"]
		data := testData.GetDataPerBuild(job, build, test, node)
		data.Perf = append(data.Perf, obj.DataItems...)

		removeStaledBuilds(testData, job, test, node, build)
	}
	return nil
}

// populateTimeSeriesData populates the time series data in testData and the
// metadata in testInfo and testTime for the given test job at the given build
// using the data fetched from source.
func populateTimeSeriesData(testData TestToBuildData, testInfo *TestInfo, testTime *TestTime, job, build string, source Downloader) error {
	contents, err := getDataFromFiles(job, build, "time_series", source)
	if err != nil {
		return err
	}
	for _, data := range contents {
		// Decode the data into obj.
		var obj nodeperftype.NodeTimeSeries
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to parse time_series data: %v\ndata=\n%v", err, string(data))
		}

		// Ignore the metrics with unsupported versions.
		if obj.Version != supportedMetricVersion {
			continue
		}

		// Populate the metadata (testInfo and testTime) with the
		// version and labels in the perf data.
		if err := populateMetadata(testInfo, testTime, obj.Labels); err != nil {
			return err
		}

		// Populate the result (testData) with the perf data.
		node := formatNodeName(obj.Labels, job)
		test := obj.Labels["test"]
		data := testData.GetDataPerBuild(job, build, test, node)
		data.Series = append(data.Series, obj)

		removeStaledBuilds(testData, job, test, node, build)
	}
	return nil
}

// populateDataForOneBuild populates perf and time series data in testData and
// test description in testinfo for the given test job at given buildNumber
// with the data fetched from source.
func populateDataForOneBuild(testData TestToBuildData, testInfo *TestInfo, job string, buildNumber int, source Downloader) error {
	build := strconv.Itoa(buildNumber)
	testTime := TestTime{}
	if err := populatePerformanceData(testData, testInfo, &testTime, job, build, source); err != nil {
		return err
	}
	if err := populateTimeSeriesData(testData, testInfo, &testTime, job, build, source); err != nil {
		return err
	}
	if *tracing {
		// Grab and convert tracing data from Kubelet log into time series data format.
		tracingData := ParseKubeletLog(source, job, buildNumber, testTime)
		// Parse time series data.
		parseTracingData(bufio.NewScanner(strings.NewReader(tracingData)), job, buildNumber, testData)
	}
	return nil
}

// State machine of the parser.
const (
	scanning   = iota
	inTest     = iota
	processing = iota
)

// parseTracingData extracts and converts tracing data into time series data.
func parseTracingData(scanner *bufio.Scanner, job string, buildNumber int, result TestToBuildData) {
	buff := &bytes.Buffer{}
	state := scanning
	build := fmt.Sprintf("%d", buildNumber)

	for scanner.Scan() {
		line := scanner.Text()
		if state == processing {
			if strings.Contains(line, timeSeriesEnd) {
				state = scanning

				obj := nodeperftype.NodeTimeSeries{}
				if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing JSON in build %d: %v\n%s\n", buildNumber, err, buff.String())
					continue
				}

				// We do not check the obj's version against
				// the supportedMetricVersion because this data
				// is generated internally by
				// ParseKubeletLog().

				testName, nodeName := obj.Labels["test"], formatNodeName(obj.Labels, job)

				if _, found := result[testName]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have no test result: %s\n", testName)
					continue
				}
				if _, found := result[testName].Data[nodeName]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have no test result: %s\n", nodeName)
					continue
				}
				if _, found := result[testName].Data[nodeName][build]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have not test result: %s\n", build)
					continue
				}

				data := result.GetDataPerBuild(job, build, testName, nodeName)
				data.Series = append(data.Series, obj)

				buff.Reset()
			}
		}
		if strings.Contains(line, timeSeriesTag) {
			state = processing
			line = line[strings.Index(line, "{"):]
		}
		if state == processing {
			buff.WriteString(line + " ")
		}
	}
}

// formatNodeName gets fromatted node name (image_machineCapacity) from labels of test data.
func formatNodeName(labels map[string]string, job string) string {
	// Get the host name of the test node.
	node := labels["node"]
	// Check if we already have the formatted name.
	if formatted, ok := nodeNameCache[node]; ok {
		return formatted
	}

	// The labels contains image and machine capacity info.
	image, okImage := labels["image"]
	machine, okMachine := labels["machine"]

	if okImage && okMachine {
		str := image + "_" + machine
		nodeNameCache[node] = str
		return str
	}

	// Can not find image/machine in the labels. Extract machine/image info
	// from host name "machine-image-uuid" (to be deprecated)
	parts := strings.Split(node, "-")
	lastPart := len(parts) - 1

	machine = parts[0] + "-" + parts[1] + "-" + parts[2]

	// GCI image name (gci-test-00-0000-0-0) is changed across build, drop the
	// suffix for daily build (000-0-0) and keep milestone (test-gci-00)
	// TODO(coufon): we should change test framework to use a consistent name.
	if job == "continuous-node-e2e-docker-benchmark" && parts[3] == "gci" {
		lastPart -= 3
	}

	result := ""
	for _, part := range parts[3:lastPart] {
		result += part + "-"
	}

	image = result[:len(result)-1]
	str := image + "_" + machine
	nodeNameCache[node] = str
	return str
}
