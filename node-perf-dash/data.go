package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/kubernetes/test/e2e/perftype"
)

const (
	// TestNameSeparator is the prefix of time name.
	TestNameSeparator = "[It] "
	// BenchmarkSeparator is the suffix of benchmark test name.
	BenchmarkSeparator = " [Benchmark]"

	// Test result tags
	PerfResultTag = perftype.PerfResultTag
	PerfResultEnd = perftype.PerfResultEnd
	// TODO(coufon): add the tags to perftype
	TimeSeriesTag = "[Result:TimeSeries]"
	TimeSeriesEnd = "[Finish:TimeSeries]"
)

// PerfData is performance test result (latency, resource-usage).
// Usually there is an array of PerfData containing different data type.
type PerfData perftype.DataItem

// SeriesData is time series data, including operation and resourage time series.
// TODO(coufon): now we only record an array of timestamps when an operation (probe)
// is called. In future we can record probe name, pod UID together with the timestamp.
// TODO(coufon): rename 'operation' to 'probe'
type SeriesData struct {
	OperationSeries map[string][]int64        `json:"op_series,omitempty"`
	ResourceSeries  map[string]ResourceSeries `json:"resource_series,omitempty"`
}

// TestData wraps up PerfData and SeriesData to simplify parser logic.
// TODO(coufon): name better json tags, need to change test code
type TestData struct {
	Version       string            `json:"version"`
	Labels        map[string]string `json:"labels,omitempty"`
	PerfDataItems []PerfData        `json:"dataItems,omitempty"`
	SeriesData
}

// ResourceSeries contains time series data
type ResourceSeries struct {
	Timestamp            []int64           `json:"ts"`
	CPUUsageInMilliCores []int64           `json:"cpu"`
	MemoryRSSInMegaBytes []int64           `json:"memory"`
	Units                map[string]string `json:"unit"`
}

// DataPerBuild contains perf data and time series for a build
type DataPerBuild struct {
	Perf   []PerfData   `json:"perf,omitempty"`
	Series []SeriesData `json:"series,omitempty"`
}

func (db *DataPerBuild) AppendPerfData(obj TestData) {
	db.Perf = append(db.Perf, obj.PerfDataItems...)
}

func (db *DataPerBuild) AppendSeriesData(obj TestData) {
	db.Series = append(db.Series, obj.SeriesData)
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
