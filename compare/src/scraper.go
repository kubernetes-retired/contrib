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

package src

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/kubernetes/test/e2e"

	"github.com/golang/glog"
)

const (
	defaultState     = iota
	readingLogs      = iota
	readingResources = iota
	readingMetrics   = iota
)

// ProcessSingleTest processes single Jenkins output file, reading and parsing JSON
// summaries embedded in it.
func ProcessSingleTest(scanner *bufio.Scanner, buildNumber int) (*e2e.LogsSizeDataSummary, *e2e.ResourceUsageSummary, *e2e.MetricsForE2E) {
	buff := &bytes.Buffer{}
	var logSummary *e2e.LogsSizeDataSummary
	var resourceSummary *e2e.ResourceUsageSummary
	var metricsSummary *e2e.MetricsForE2E
	state := defaultState
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Finished") {
			if state == readingLogs {
				logSummary = &e2e.LogsSizeDataSummary{}
				state = defaultState
				if err := json.Unmarshal(buff.Bytes(), logSummary); err != nil {
					glog.V(0).Infof("error parsing LogsSizeDataSummary JSON in build %d: %v %s\n", buildNumber, err, buff.String())
					continue
				}
			}
			if state == readingResources {
				resourceSummary = &e2e.ResourceUsageSummary{}
				state = defaultState
				if err := json.Unmarshal(buff.Bytes(), resourceSummary); err != nil {
					glog.V(0).Infof("error parsing ResourceUsageSummary JSON in build %d: %v %s\n", buildNumber, err, buff.String())
					continue
				}
			}
			if state == readingMetrics {
				metricsSummary = &e2e.MetricsForE2E{}
				state = defaultState
				if err := json.Unmarshal(buff.Bytes(), metricsSummary); err != nil {
					glog.V(0).Infof("error parsing MetricsForE2E JSON in build %d: %v %s\n", buildNumber, err, buff.String())
					continue
				}
			}
			buff.Reset()
		}
		if state != defaultState {
			buff.WriteString(line + " ")
		}
		if strings.Contains(line, "LogsSizeDataSummary JSON") {
			state = readingLogs
		}
		if strings.Contains(line, "ResourceUsageSummary JSON") {
			state = readingResources
		}
		if strings.Contains(line, "MetricsForE2E JSON") {
			state = readingMetrics
		}
	}
	return logSummary, resourceSummary, metricsSummary
}
