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
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/kubernetes/test/e2e/perftype"
	node_perftype "k8s.io/kubernetes/test/e2e_node/perftype"
)

// DataPerBuild contains perf/time series data for a build.
type DataPerBuild struct {
	Perf   []perftype.DataItem            `json:"perf,omitempty"`
	Series []node_perftype.NodeTimeSeries `json:"series,omitempty"`
}

// DataPerNode contains perf/time series data for a node.
type DataPerNode map[string]*DataPerBuild

// DataPerTest contains perf/time series data for a test.
type DataPerTest struct {
	Data map[string]DataPerNode `json:"data"`
	Job  string                 `json:"job"`
}

// TestToBuildData is a map from job name to DataPerTest.
type TestToBuildData map[string]*DataPerTest

// GetDataPerBuild creates a DataPerBuild structure for the given build using
// the specified job, test, and node if it does not exist, and then returns it.
func (b TestToBuildData) GetDataPerBuild(job, build, test, node string) *DataPerBuild {
	if _, ok := b[test]; !ok {
		b[test] = &DataPerTest{
			Job:  job,
			Data: map[string]DataPerNode{},
		}
	}
	if _, ok := b[test].Data[node]; !ok {
		b[test].Data[node] = DataPerNode{}
	}
	if _, ok := b[test].Data[node][build]; !ok {
		b[test].Data[node][build] = &DataPerBuild{}
	}
	return b[test].Data[node][build]
}

// ServeHTTP is the HTTP handler for serving TestToBuildData.
func (b TestToBuildData) ServeHTTP(res http.ResponseWriter, req *http.Request) {
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

// TestInfo contains the mapping from test name to test description.
type TestInfo struct {
	// Info is a map from test name to test description.
	// For example,
	// "resource_0" -> "resource tracking for 0 pods per node [Benchmark]".
	Info map[string]string `json:"info"`
}

// ServeHTTP is the HTTP handler for serving TestInfo.
func (b *TestInfo) ServeHTTP(res http.ResponseWriter, req *http.Request) {
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
