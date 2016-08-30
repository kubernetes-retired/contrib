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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
)

const (
	// TODO(coufon): move to and then grab from pkg/kubelet/tracing/tracing.go
	tracingVersion      = "v1"
	tracingEventReason  = "NodeTracing"
	timeSeriesResultTag = "[Result:TimeSeries]"
	timeSeriesFinishTag = "[Finish:TimeSeries]"
)

var (
	regexEvent     = regexp.MustCompile(`[IW](?:\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}) .* Event\(.*\): type: \'(.*)\' reason: \'(.*)\' (.*)`)
	regexEventMsg  = regexp.MustCompile(`pod: (.*), probe: (.*), timestamp: ([0-9]*)`)
	regexTestStart = regexp.MustCompile(`(?:.*): INFO: The test (.*) on (.*) starts at ([0-9]*).`)
	regexTestEnd   = regexp.MustCompile(`(?:.*): INFO: The test (.*) on (.*) ends at ([0-9]*).`)
)

// TestTimeRange records the start and end timestamp of a test.
type TestTimeRange struct {
	Test      string
	Node      string
	StartTime int64
	EndTime   int64
}

// InRange checks whether a timestamp is within the test time range.
func (ttr *TestTimeRange) InRange(ts int64) bool {
	if ts >= ttr.StartTime && ts <= ttr.EndTime {
		return true
	}
	return false
}

// GrabTestTimeRange return a list of tests and their time ranges.
func GrabTestTimeRange(d Downloader, job string, buildNumber int) []*TestTimeRange {
	var ttrList []*TestTimeRange

	file, err := d.GetFile(job, buildNumber, testResultFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while fetching tracing data time range: %v\n", err)
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var ttr *TestTimeRange
	for scanner.Scan() {
		line := scanner.Text()
		// Match start time of test
		matchResult := regexTestStart.FindSubmatch([]byte(line))
		if matchResult != nil {
			if ttr == nil {
				timestamp, err := strconv.ParseInt(string(matchResult[3]), 10, 64)
				if err != nil {
					log.Fatal(err)
				}
				ttr = &TestTimeRange{
					Test:      string(matchResult[1]),
					Node:      string(matchResult[2]),
					StartTime: timestamp,
				}
			} else {
				log.Fatal("Error: find unhandled test start timestamp")
			}
			continue
		}
		// Match end time of test
		matchResult = regexTestEnd.FindSubmatch([]byte(line))
		if matchResult != nil {
			test, node := string(matchResult[1]), string(matchResult[2])
			timestamp, err := strconv.ParseInt(string(matchResult[3]), 10, 64)
			if err != nil {
				log.Fatal(err)
			}
			if ttr != nil && ttr.Test == test && ttr.Node == node && ttr.EndTime == 0 {
				ttr.EndTime = timestamp
				ttrList = append(ttrList, ttr)
				ttr = nil
			} else {
				log.Fatal("Error: start and end timestamps mismatch")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return ttrList
}

// TracingData contains the tracing time series data of a test on a node
type TracingData struct {
	Labels  map[string]string   `json:"labels"`
	Version string              `json:"version"`
	Data    map[string]int64arr `json:"op_series"`
}

// AppendData adds a new timestamp of a probe into tracing data.
func (td *TracingData) AppendData(probe string, timestamp int64) {
	if _, ok := td.Data[probe]; !ok {
		td.Data[probe] = make([]int64, 0)
	}
	td.Data[probe] = append(td.Data[probe], timestamp)
}

// SortData have all time series data sorted
func (td *TracingData) SortData() {
	for _, arr := range td.Data {
		sort.Sort(arr)
	}
}

// ToSeriesData returns stringified tracing data in JSON
func (td *TracingData) ToSeriesData() string {
	seriesData, err := json.Marshal(td)
	if err != nil {
		log.Fatal(err)
	}
	return string(seriesData)
}

// GrabTracing parses tracing events in kubelet.log and filter events by time range.
func GrabTracing(d Downloader, job string, buildNumber int, ttr *TestTimeRange) string {
	file, err := d.GetFile(job, buildNumber, path.Join("artifacts", ttr.Node, kubeletLogFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while fetching tracing data event: %v\n", err)
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	// TODO(coufon): we do not store pod name for each timestamp.
	tracingData := TracingData{
		Labels: map[string]string{
			"test": ttr.Test,
			"node": ttr.Node,
		},
		Version: tracingVersion,
		Data:    map[string]int64arr{},
	}
	for scanner.Scan() {
		line := scanner.Text()
		matchResult := regexEvent.FindSubmatch([]byte(line))
		// Find a tracing event in kubelet log
		if matchResult != nil {
			eventReason := string(matchResult[2])
			if eventReason == tracingEventReason {
				//eventType := string(matchResult[1])
				eventMsg := matchResult[3]
				matchResult = regexEventMsg.FindSubmatch(eventMsg)
				if matchResult != nil {
					//pod := string(matchResult[1])
					probe := string(matchResult[2])
					timestamp, err := strconv.ParseInt(string(matchResult[3]), 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					if ttr.InRange(timestamp) {
						tracingData.AppendData(probe, timestamp)
					}
				} else {
					log.Fatalf("Error: parsing event message error :%s\n", eventMsg)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	tracingData.SortData()

	return timeSeriesResultTag + tracingData.ToSeriesData() + "\n\n" +
		timeSeriesFinishTag + "\n"
}

// ParseTracing parses tracing data from kubelet.log.
// It firstly grabs the start and end time of each test from build-log.txt,
// then use this time range to extract tracing events for each test.
func ParseTracing(d Downloader, job string, buildNumber int) string {
	ttrList := GrabTestTimeRange(d, job, buildNumber)
	result := ""
	for _, ttr := range ttrList {
		result += GrabTracing(d, job, buildNumber, ttr)
	}
	return result
}

type int64arr []int64

func (a int64arr) Len() int           { return len(a) }
func (a int64arr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a int64arr) Less(i, j int) bool { return a[i] < a[j] }
