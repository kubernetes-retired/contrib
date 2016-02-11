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

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	// TODO: move this somewhere central
	"k8s.io/kubernetes/pkg/util/sets"
)

// LatencyData represents the latency data for a set of RESTful API calls
type LatencyData struct {
	APICalls []APICallLatency `json:"apicalls"`
}

// APICallLatency represents the latency data for a (resource, verb) tuple
type APICallLatency struct {
	Latency  Histogram `json:"latency"`
	Resource string    `json:"resource"`
	Verb     string    `json:"verb"`
}

// Histogram is a map from bucket to latency (e.g. "Perc90" -> 23.5)
type Histogram map[string]float64

// ResourceToHistogram is a map from resource names (e.g. "pods") to the relevant latency data
type ResourceToHistogram map[string][]APICallLatency

// TestToHistogram is a map from test name to ResourceToHistogram
type TestToHistogram map[string]ResourceToHistogram

// BuildLatencyData is a map from build number to latency data
type BuildLatencyData map[string]ResourceToHistogram

// TestToBuildData is a map from test name to BuildLatencyData
type TestToBuildData map[string]BuildLatencyData

func (buildLatency *TestToBuildData) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(buildLatency)
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

// states of parsing machine
const (
	scanning   = iota
	inTest     = iota
	processing = iota
)

// constants to use for downloading data.
const (
	urlPrefix = "https://storage.googleapis.com/kubernetes-jenkins/logs/kubernetes-e2e-gce-scalability/"
	urlSuffix = "/build-log.txt"
)

var descriptionToName = map[string]string{
	"should allow starting 30 pods per node":    "Density",
	"should be able to handle 30 pods per node": "Load",
}

// Assumes that *resources* and *methods* are already initialized.
func parseTestOutput(scanner *bufio.Scanner, buildNumber int, resources sets.String, methods sets.String) TestToHistogram {
	buff := &bytes.Buffer{}
	hist := TestToHistogram{}
	state := scanning
	testNameSeparator := "[It] [Feature:Performance]"
	testName := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, testNameSeparator) {
			state = inTest
			var ok bool
			testName, ok = descriptionToName[strings.Trim(strings.Split(line, testNameSeparator)[1], " ")]
			if !ok {
				testName = "Unknown"
			}
			hist[testName] = make(ResourceToHistogram)
		}
		if state == processing {
			// TODO: This is brittle, we should emit a tail delimiter too
			if strings.Contains(line, "INFO") || strings.Contains(line, "STEP") || strings.Contains(line, "Failure") || strings.Contains(line, "[AfterEach]") {
				obj := LatencyData{}
				if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing JSON in build %d: %v %s\n", buildNumber, err, buff.String())
					// reset state and try again with more input
					state = scanning
					continue
				}

				for _, call := range obj.APICalls {
					hist[testName][call.Resource] = append(hist[testName][call.Resource], call)
					resources.Insert(call.Resource)
					methods.Insert(call.Verb)
				}

				buff.Reset()
				state = scanning
			}
		}
		if state == inTest && strings.Contains(line, "API calls latencies") {
			state = processing
			line = line[strings.Index(line, "{"):]
		}
		if state == processing {
			buff.WriteString(line + " ")
		}
	}
	return hist
}

func getLatencyDataFormGCS() (TestToBuildData, sets.String, sets.String, error) {
	fmt.Print("Getting Data from GCS...\n")
	buildLatency := TestToBuildData{}
	resources := sets.NewString()
	methods := sets.NewString()

	buildNumber := *startFrom
	latestBuildResponse, err := http.Get(fmt.Sprintf("%vlatest-build.txt", urlPrefix))
	if err != nil {
		return buildLatency, resources, methods, err
	}
	latestBuildBody := latestBuildResponse.Body
	defer latestBuildBody.Close()
	latestBuildBodyScanner := bufio.NewScanner(latestBuildBody)
	latestBuildBodyScanner.Scan()
	var lastBuildNo int
	fmt.Sscanf(latestBuildBodyScanner.Text(), "BUILD_NUMBER=%d", &lastBuildNo)
	fmt.Printf("Last build no: %v\n", lastBuildNo)
	if buildNumber < lastBuildNo-100 {
		buildNumber = lastBuildNo - 100
	}

	for ; buildNumber <= lastBuildNo; buildNumber++ {
		fmt.Printf("Fetching build %v...\n", buildNumber)
		testDataResponse, err := http.Get(fmt.Sprintf("%v%v%v", urlPrefix, buildNumber, urlSuffix))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while fetching data: %v\n", err)
			continue
		}

		testDataBody := testDataResponse.Body
		defer testDataBody.Close()
		testDataScanner := bufio.NewScanner(testDataBody)

		hist := parseTestOutput(testDataScanner, buildNumber, resources, methods)

		for k, v := range hist {
			if _, ok := buildLatency[k]; !ok {
				buildLatency[k] = make(BuildLatencyData)
			}
			buildLatency[k][fmt.Sprintf("%d", buildNumber)] = v
		}
	}

	return buildLatency, resources, methods, nil
}

func generateCSV(buildLatency BuildLatencyData, resources, methods sets.String, out io.Writer) error {
	header := []string{"build"}
	for _, rsrc := range resources.List() {
		header = append(header, fmt.Sprintf("%s_50", rsrc))
		header = append(header, fmt.Sprintf("%s_90", rsrc))
		header = append(header, fmt.Sprintf("%s_99", rsrc))
	}
	if _, err := fmt.Fprintln(out, strings.Join(header, ",")); err != nil {
		return err
	}

	for _, method := range methods.List() {
		if _, err := fmt.Fprintln(out, method); err != nil {
			return err
		}
		for build, data := range buildLatency {
			line := []string{fmt.Sprintf("%d", build)}
			for _, rsrc := range resources.List() {
				podData := data[rsrc]
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc50", podData)))
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc90", podData)))
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc99", podData)))
			}
			if _, err := fmt.Fprintln(out, strings.Join(line, ",")); err != nil {
				return err
			}
		}
	}
	return nil
}

func findMethod(method, item string, data []APICallLatency) float64 {
	for _, datum := range data {
		if datum.Verb == method {
			return datum.Latency[item]
		}
	}
	return -1
}

var (
	www         = flag.Bool("www", false, "If true, start a web-server to server performance data")
	addr        = flag.String("address", ":8080", "The address to serve web data on, only used if -www is true")
	wwwDir      = flag.String("dir", "", "If non-empty, add a file server for this directory at the root of the web server")
	jenkinsHost = flag.String("jenkins-host", "", "The URL for the jenkins server.")
	startFrom   = flag.Int("start-from", 0, "First build number to include in the results")

	pollDuration = 10 * time.Minute
	errorDelay   = 10 * time.Second
)

func main() {
	fmt.Print("Starting perfdash...\n")
	flag.Parse()

	if !*www {
		buildLatency, resources, methods, err := getLatencyDataFormGCS()
		if err != nil {
			fmt.Printf("Failed to get data: %v\n", err)
			os.Exit(1)
		}
		for _, v := range buildLatency {
			generateCSV(v, resources, methods, os.Stdout)
		}
		return
	}

	buildLatency := TestToBuildData{}
	resources := sets.String{}
	methods := sets.String{}
	var err error
	go func() {
		for {
			fmt.Printf("Fetching new data...\n")
			buildLatency, resources, methods, err = getLatencyDataFormGCS()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
				time.Sleep(errorDelay)
				continue
			}
			time.Sleep(pollDuration)
		}
	}()

	http.Handle("/api", &buildLatency)
	http.Handle("/", http.FileServer(http.Dir(*wwwDir)))
	http.ListenAndServe(*addr, nil)
}
