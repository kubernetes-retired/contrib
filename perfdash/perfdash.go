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
	"k8s.io/contrib/mungegithub/pulls/jenkins"
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

// BuildLatencyData is a map from build number to latency data
type BuildLatencyData map[string]ResourceToHistogram

func (buildLatency *BuildLatencyData) ServeHTTP(res http.ResponseWriter, req *http.Request) {
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

func getLatencyData(client *jenkins.JenkinsClient, job string) (BuildLatencyData, sets.String, sets.String, error) {
	buildLatency := BuildLatencyData{}
	resources := sets.NewString()
	methods := sets.NewString()

	queue, err := client.GetJob(job)
	if err != nil {
		return buildLatency, resources, methods, err
	}

	for ix := range queue.Builds {
		build := queue.Builds[ix]
		reader, err := client.GetConsoleLog(job, build.Number)
		if err != nil {
			fmt.Printf("error getting logs: %v", err)
			continue
		}
		defer reader.Close()
		scanner := bufio.NewScanner(reader)
		buff := &bytes.Buffer{}
		inLatency := false

		hist := ResourceToHistogram{}
		found := false
		testNameSeparator := "[It] [Skipped] [Performance suite]"
		testName := ""
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, testNameSeparator) {
				testName = strings.Trim(strings.Split(line, testNameSeparator)[1], " ")
			}
			// TODO: This is brittle, we should emit a tail delimiter too
			if strings.Contains(line, "INFO") || strings.Contains(line, "STEP") || strings.Contains(line, "Failure") {
				if inLatency {
					obj := LatencyData{}
					if err := json.Unmarshal(buff.Bytes(), &obj); err != nil {
						fmt.Printf("error parsing JSON in build %d: %v %s\n", build.Number, err, buff.String())
						// reset state and try again with more input
						inLatency = false
						continue
					}

					if testName == "should allow starting 30 pods per node" {
						for _, call := range obj.APICalls {
							list := hist[call.Resource]
							list = append(list, call)
							hist[call.Resource] = list
							resources.Insert(call.Resource)
							methods.Insert(call.Verb)
						}
					}

					buff.Reset()
				}
				inLatency = false
			}
			if strings.Contains(line, "API calls latencies") {
				found = true
				inLatency = true
				ix = strings.Index(line, "{")
				line = line[ix:]
			}
			if inLatency {
				buff.WriteString(line + " ")
			}
		}
		if !found {
			continue
		}

		buildLatency[fmt.Sprintf("%d", build.Number)] = hist
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

	pollDuration = 10 * time.Minute
	errorDelay   = 10 * time.Second
)

func main() {
	flag.Parse()
	job := "kubernetes-e2e-gce-scalability"
	client := &jenkins.JenkinsClient{
		Host: *jenkinsHost,
	}

	if !*www {
		buildLatency, resources, methods, err := getLatencyData(client, job)
		if err != nil {
			fmt.Printf("Failed to get data: %v\n", err)
			os.Exit(1)
		}
		generateCSV(buildLatency, resources, methods, os.Stdout)
		return
	}

	buildLatency := BuildLatencyData{}
	resources := sets.String{}
	methods := sets.String{}
	var err error
	go func() {
		for {
			buildLatency, resources, methods, err = getLatencyData(client, job)
			if err != nil {
				fmt.Printf("Error fetching data: %v\n", err)
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
