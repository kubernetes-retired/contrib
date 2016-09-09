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
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	pollDuration = 10 * time.Minute
	errorDelay   = 10 * time.Second
	maxBuilds    = 100
)

var (
	addr         = flag.String("address", ":8080", "The address to serve web data on")
	www          = flag.Bool("www", true, "If true, start a web-server to server performance data")
	wwwDir       = flag.String("dir", "www", "If non-empty, add a file server for this directory at the root of the web server")
	builds       = flag.Int("builds", maxBuilds, "Total builds number")
	datasource   = flag.String("datasource", "google-gcs", "Source of test data. Options include 'local', 'google-gcs'")
	localDataDir = flag.String("local-data-dir", "", "The path to test data directory")
	tracing      = flag.Bool("tracing", false, "If true, try to obtain tracing data from kubelet.log")
	jenkinsJob   = flag.String("jenkins-job", "kubelet-benchmark-gce-e2e-ci", "Name of the Jenkins job running the tests")
)

func main() {
	var (
		downloader Downloader
		err        error
		jobs       JobList
	)

	fmt.Print("Starting Node Performance Dashboard...\n")
	flag.Parse()

	if *builds > maxBuilds || *builds < 0 {
		fmt.Printf("Invalid builds number: %d", *builds)
		*builds = maxBuilds
	}

	switch *datasource {
	case "local":
		downloader = NewLocalDownloader()
	case "google-gcs":
		downloader = NewGoogleGCSDownloader()
	default:
		fmt.Fprintf(os.Stderr, "Unsupported data source: %s.\n", *datasource)
		os.Exit(1)
	}

	jobs = strings.Split(*jenkinsJob, ",")
	fmt.Println(jobs)
	for _, job := range jobs {
		allTestData[job] = &TestToBuildData{}
	}

	if !*www {
		for _, job := range jobs {
			err = GetData(job, downloader)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
				os.Exit(1)
			}
			prettyResult, err := json.MarshalIndent(allTestData, "", " ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formating data: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Result: %v\n", string(prettyResult))
		}
		return
	}

	for _, job := range jobs {
		go func(job string) {
			for {
				fmt.Printf("Fetching new data...\n")
				err = GetData(job, downloader)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
					time.Sleep(errorDelay)
					continue
				}
				time.Sleep(pollDuration)
			}
		}(job)
	}

	for _, job := range jobs {
		http.Handle(fmt.Sprintf("/data/%s", job), allTestData[job])
	}
	http.Handle("/testinfo", &testInfoMap)
	http.Handle("/jobs", &jobs)
	http.Handle("/", http.FileServer(http.Dir(*wwwDir)))
	http.ListenAndServe(*addr, nil)
}

// JobList is the list containing all job names
type JobList []string

func (j *JobList) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(j)
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
