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
	"fmt"
	"strings"

	// TODO: move this somewhere central
	"k8s.io/contrib/submit-queue/jenkins"
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

type resourceToHistogram map[string][]APICallLatency

var buildLatency = map[int]resourceToHistogram{}

func main() {
	job := "kubernetes-e2e-gce-scalability"
	client := jenkins.JenkinsClient{
		Host: "http://kubekins.dls.corp.google.com",
	}

	queue, err := client.GetJob(job)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	resources := sets.NewString()
	methods := sets.NewString()
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

		hist := resourceToHistogram{}
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

		buildLatency[build.Number] = hist
	}

	header := []string{"build"}
	for _, rsrc := range resources.List() {
		header = append(header, fmt.Sprintf("%s_50", rsrc))
		header = append(header, fmt.Sprintf("%s_90", rsrc))
		header = append(header, fmt.Sprintf("%s_99", rsrc))
	}
	fmt.Println(strings.Join(header, ","))

	for _, method := range methods.List() {
		fmt.Println(method)
		for build, data := range buildLatency {
			line := []string{fmt.Sprintf("%d", build)}
			for _, rsrc := range resources.List() {
				podData := data[rsrc]
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc50", podData)))
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc90", podData)))
				line = append(line, fmt.Sprintf("%g", findMethod(method, "Perc99", podData)))
			}
			fmt.Println(strings.Join(line, ","))
		}
	}
}

func findMethod(method, item string, data []APICallLatency) float64 {
	for _, datum := range data {
		if datum.Verb == method {
			return datum.Latency[item]
		}
	}
	return -1
}
