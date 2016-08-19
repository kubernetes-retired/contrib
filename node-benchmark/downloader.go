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
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/kubernetes/test/e2e/perftype"
)

// Downloader is the interface that gets a data from a predefined source.
type Downloader interface {
	getData() (TestToBuildData, error)
}

// DataPerBuild contains perf data and time series for a build
type DataPerBuild struct {
	PerfData   []perftype.DataItem `json:"perf,omitempty"`
	SeriesData []TestData          `json:"series,omitempty"`
}

func (db *DataPerBuild) AppendPerfData(obj TestData) {
	db.PerfData = append(db.PerfData, obj.DataItems...)
}

func (db *DataPerBuild) AppendSeriesData(obj TestData) {
	db.SeriesData = append(db.SeriesData, obj)
}

// DataPerNode contains perf data and time series for a node
type DataPerNode map[string]*DataPerBuild

// BuildData contains job name and a map from build number to perf data
type DataPerTest struct {
	Data    map[string]DataPerNode `json:"data"`
	Job     string                 `json:"job"`
	Version string                 `json:"version"`
}

// TestToBuildData is a map from test name to BuildData
// TODO(random-liu): Use a more complex data structure if we need to support more test in the future.
type TestToBuildData map[string]*DataPerTest

func (b *TestToBuildData) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(b)
	if err != nil {
		res.Header().Set("Content-type", "text/html")
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(fmt.Sprintf("<h3>Internal Error</h3><p>%v", err)))
		return
	}
	res.Header().Set("Content-type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(data)
}

const (
	TimeSeriesTag = "[Result:TimeSeries]"
	TimeSeriesEnd = "[Finish:TimeSeries]"
)

// ResourceSeries contains time series data
type ResourceSeries struct {
	Timestamp            []int64           `json:"ts"`
	CPUUsageInMilliCores []int64           `json:"cpu"`
	MemoryRSSInMegaBytes []int64           `json:"memory"`
	Units                map[string]string `json:"unit"`
}

// PerfData contains all data items generated in current test.
type TestData struct {
	Version         string                    `json:"version"`
	DataItems       []perftype.DataItem       `json:"dataItems,omitempty"`
	OperationSeries map[string][]int64        `json:"op_data,omitempty"`
	ResourceSeries  map[string]ResourceSeries `json:"resource_data,omitempty"`
	Labels          map[string]string         `json:"labels,omitempty"`
}

// TestInfoMap contains all testInfo indexed by short test name
type TestInfoMap struct {
	Info map[string]string `json:"info"`
}

var testInfoMap = TestInfoMap{
	Info: make(map[string]string),
}

func (b *TestInfoMap) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(b)
	if err != nil {
		res.Header().Set("Content-type", "text/html")
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(fmt.Sprintf("<h3>Internal Error</h3><p>%v", err)))
		return
	}
	res.Header().Set("Content-type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(data)
}
