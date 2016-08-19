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
	"os"
	"strings"

	"k8s.io/kubernetes/test/e2e/perftype"
)

// states of parsing machine
const (
	scanning   = iota
	inTest     = iota
	processing = iota
)

func parseTestOutput(scanner *bufio.Scanner, job string, tests Tests, buildNumber int, result TestToBuildData) {
	buff := &bytes.Buffer{}
	state := scanning
	//testName := ""
	TestDetail := ""
	build := fmt.Sprintf("%d", buildNumber)

	isTimeSeries := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, TestNameSeparator) && strings.Contains(line, BenchmarkSeparator) {
			TestDetail = line //strings.Trim(strings.Split(line, TestNameSeparator)[1], BenchmarkSeparator)
			//var ok bool
			//testName, ok = tests[strings.Trim(strings.Split(line, TestNameSeparator)[1], " ")]
			//ok := strings.Contains(line, BenchmarkSeparator)
			//if !ok {
			//	state = scanning
			//	continue
			//}
			state = inTest
			//if _, found := result[testName]; found && result[testName].Job != job {
			// If the job is different, we'll skip the test and keep the old result
			//	state = scanning
			//}
		}
		if state == processing {
			// TODO(random-liu): Deprecate the old label when all latest 100 builds have the perftype.PerfResultEnd
			if strings.Contains(line, perftype.PerfResultEnd) || strings.Contains(line, TimeSeriesEnd) || strings.Contains(line, "INFO") ||
				strings.Contains(line, "STEP") || strings.Contains(line, "Failure") || strings.Contains(line, "[AfterEach]") {
				state = inTest

				obj := TestData{}
				if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing JSON in build %d: %v %s\n", buildNumber, err, buff.String())
					continue
				}
				testName, nodeName := obj.Labels["test"], obj.Labels["node"]
				nodeName = strings.Split(nodeName, "-image-")[0]
				testInfoMap.Info[testName] = TestDetail

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
				if _, found := result[testName].Data[nodeName][build]; !found {
					result[testName].Data[nodeName][build] = &DataPerBuild{}
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
		if state == inTest && (strings.Contains(line, perftype.PerfResultTag) || strings.Contains(line, TimeSeriesTag)) {
			if strings.Contains(line, TimeSeriesTag) {
				isTimeSeries = true
			}
			state = processing
			line = line[strings.Index(line, "{"):]
		}
		if state == processing {
			buff.WriteString(line + " ")
		}
	}
}
