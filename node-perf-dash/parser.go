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
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// states of parsing machine
const (
	scanning   = iota
	inTest     = iota
	processing = iota
)

var (
	// Regex for the performance result data log entry. It is used to parse the test end time.
	regexResult = regexp.MustCompile(`([A-Z][a-z]*\s{1,2}\d{1,2} \d{2}:\d{2}:\d{2}.\d{3}): INFO: .*`)
	// map[testName + nodeName string]FIFO(build string)
	buildFIFOs = map[string][]string{}
)

func parseTestOutput(scanner *bufio.Scanner, job string, buildNumber int, result TestToBuildData,
	testTime TestTime) {
	buff := &bytes.Buffer{}
	state := scanning
	TestDetail := ""
	endTime := ""
	build := fmt.Sprintf("%d", buildNumber)

	isTimeSeries := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, TestNameSeparator) && strings.Contains(line, BenchmarkSeparator) {
			TestDetail = line
			state = inTest
		}
		if state == processing {
			if strings.Contains(line, perfResultEnd) || strings.Contains(line, timeSeriesEnd) ||
				strings.Contains(line, "INFO") || strings.Contains(line, "STEP") ||
				strings.Contains(line, "Failure") || strings.Contains(line, "[AfterEach]") {
				state = inTest

				obj := TestData{}
				if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
					fmt.Fprintf(os.Stderr, "Error: parsing JSON in build %d: %v %s\n",
						buildNumber, err, buff.String())
					continue
				}

				testName, nodeName := obj.Labels["test"], obj.Labels["node"]
				testInfoMap.Info[testName] = TestDetail

				if endTime == "" {
					log.Fatal("Error: test end time not parsed")
				}

				testTime.Add(testName, nodeName, endTime)
				endTime = "" // reset

				// remove suffix
				nodeName = formatNodeName(nodeName, job)

				if _, found := result[testName]; !found {
					result[testName] = &DataPerTest{
						Job:     job,
						Version: obj.Version,
						Data:    map[string]DataPerNode{},
					}
				}
				if _, found := result[testName].Data[nodeName]; !found {
					result[testName].Data[nodeName] = DataPerNode{}
				}
				// Find data from a new build.
				if _, found := result[testName].Data[nodeName][build]; !found {
					result[testName].Data[nodeName][build] = &DataPerBuild{}

					key := testName + "/" + nodeName
					// Update build FIFO
					buildFIFOs[key] = append(buildFIFOs[key], build)
					// Remove stale builds
					if len(buildFIFOs[key]) > *builds {
						delete(result[testName].Data[nodeName], buildFIFOs[key][0])
						buildFIFOs[key] = buildFIFOs[key][1:]
					}
				}

				if result[testName].Version == obj.Version {
					if isTimeSeries {
						(result[testName].Data[nodeName][build]).AppendSeriesData(obj)
						isTimeSeries = false
					} else {
						(result[testName].Data[nodeName][build]).AppendPerfData(obj)
					}
				}

				buff.Reset()
			}
		}
		if state == inTest && (strings.Contains(line, perfResultTag) || strings.Contains(line, timeSeriesTag)) {
			if strings.Contains(line, timeSeriesTag) {
				isTimeSeries = true
			}
			state = processing

			// Parse test end time
			matchResult := regexResult.FindSubmatch([]byte(line))
			if matchResult != nil {
				endTime = string(matchResult[1])
			} else {
				log.Fatalf("Error: can not parse test end time:\n%s\n", line)
			}

			line = line[strings.Index(line, "{"):]
		}
		if state == processing {
			buff.WriteString(line + " ")
		}
	}
}

// formatNodeName the UUID suffix of node name.
func formatNodeName(nodeName string, job string) string {
	result := ""
	parts := strings.Split(nodeName, "-")
	lastPart := len(parts) - 1

	// TODO(coufon): GCI image name is changed across build, we should
	// change test framework to use a consistent name.\
	if job == "continuous-node-e2e-docker-benchmark" && parts[3] == "gci" {
		lastPart -= 3
	}

	for _, part := range parts[0:lastPart] {
		result += part + "-"
	}
	return result[:len(result)-1]
}

// parseTracingData extracts and appends tracing data into time series data.
func parseTracingData(scanner *bufio.Scanner, job string, buildNumber int, result TestToBuildData) {
	buff := &bytes.Buffer{}
	state := scanning
	build := fmt.Sprintf("%d", buildNumber)

	for scanner.Scan() {
		line := scanner.Text()
		if state == processing {
			if strings.Contains(line, timeSeriesEnd) {
				state = scanning

				obj := TestData{}
				if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing JSON in build %d: %v\n%s\n", buildNumber, err, buff.String())
					continue
				}
				testName, nodeName := obj.Labels["test"], obj.Labels["node"]
				nodeName = formatNodeName(nodeName, job)

				if _, found := result[testName]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have no test result: %s", testName)
					continue
				}
				if _, found := result[testName].Data[nodeName]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have not test result: %s", nodeName)
					continue
				}
				if _, found := result[testName].Data[nodeName][build]; !found {
					fmt.Fprintf(os.Stderr, "Error: tracing data have not test result: %s", build)
					continue
				}

				if result[testName].Version == obj.Version {
					(result[testName].Data[nodeName][build]).AppendSeriesData(obj)
				}

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
