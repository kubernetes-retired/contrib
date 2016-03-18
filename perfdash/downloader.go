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
	"k8s.io/kubernetes/pkg/util/sets"
)

// Histogram is a map from bucket to latency (e.g. "Perc90" -> 23.5)
type Histogram map[string]float64

// APICallLatency represents the latency data for a (resource, verb) tuple
type APICallLatency struct {
	Latency  Histogram `json:"latency"`
	Resource string    `json:"resource"`
	Verb     string    `json:"verb"`
}

// LatencyData represents the latency data for a set of RESTful API calls
type LatencyData struct {
	APICalls []APICallLatency `json:"apicalls"`
}

// ResourceToHistogram is a map from resource names (e.g. "pods") to the relevant latency data
type ResourceToHistogram map[string][]APICallLatency

// TestToHistogram is a map from test name to ResourceToHistogram
type TestToHistogram map[string]ResourceToHistogram

// BuildLatencyData is a map from build number to latency data
type BuildLatencyData map[string]ResourceToHistogram

// TestToBuildData is a map from test name to BuildLatencyData
type TestToBuildData map[string]BuildLatencyData

// Downloader is the interface that gets a data from a predefined source.
type Downloader interface {
	getData() (TestToBuildData, sets.String, sets.String, error)
}
