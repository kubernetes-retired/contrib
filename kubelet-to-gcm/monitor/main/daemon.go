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
	"fmt"
	"os"
	"time"

	log "github.com/golang/glog"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	v3 "google.golang.org/api/monitoring/v3"
	"k8s.io/contrib/kubelet-to-gcm/monitor"
)

const (
	missingValue = "/MISSING"
	scope        = "https://www.googleapis.com/auth/monitoring.write"
	testPath     = "https://test-monitoring.sandbox.googleapis.com"
)

var (
	// Flags to identify the Kubelet.
	zone    = flag.String("zone", "us-central1-b", "The zone where this kubelet lives.")
	project = flag.String("project", missingValue, "The project where this kubelet's host lives.")
	cluster = flag.String("cluster", missingValue, "The cluster where this kubelet holds membership.")
	host    = flag.String("host", "localhost", "The kubelet's host name.")
	port    = flag.Uint("port", 10255, "The kubelet's port.")
	// Flags to control runtime behavior.
	resolution = time.Second * time.Duration(*flag.Uint("resolution", 10, "The time, in seconds, to poll the Kubelet."))
	useTest    = flag.Bool("use-test", false, "If the test GCM endpoint should be used.")
)

func main() {
	// First log our starting config, and then set up.
	defer log.Flush()
	log.Infof("Invoked by %v", os.Args)
	flag.Parse()

	// We can't infer cluster, so it must be specified.
	if *cluster == missingValue {
		log.Fatalf("The cluster must be specified: %v", os.Args)
	}

	translator := monitor.NewTranslator(*zone, *project, *cluster, *host, resolution)
	log.Info("New Translator successfully created.")

	// NewKubeletClient validates its own inputs.
	kubelet, err := monitor.NewKubeletClient(*host, *port, nil)
	if err != nil {
		log.Fatalf("Failed to create a Kubelet client with host %s and port %d: %v", *host, *port, err)
	}
	log.Info("Successfully created kubelet client.")

	// Create a GCM client.
	name := fmt.Sprintf("projects/%s", *project)
	client, err := google.DefaultClient(context.Background(), scope)
	if err != nil {
		log.Fatalf("Failed to create a client with default context and scope %s, err: %v", scope, err)
	}
	service, err := v3.New(client)
	if err != nil {
		log.Fatalf("Failed to create a GCM v3 API service object: %v", err)
	}
	if *useTest {
		service.BasePath = testPath
	}

	for {
		go func() {
			// Get the latest summary.
			summary, err := kubelet.GetSummary()
			if err != nil {
				log.Errorf("Failed to get summary from kubelet: %v", err)
				return
			}

			// Translate kubelet's data to GCM v3's format.
			tsReq, err := translator.Translate(summary)
			if err != nil {
				log.Errorf("Failed to translate data from summary %v: %v", summary, err)
				return
			}

			// Push that data to GCM's v3 API.
			createCall := service.Projects.TimeSeries.Create(name, tsReq)
			if empty, err := createCall.Do(); err != nil {
				log.Errorf("Failed to write time series data, empty: %v, err: %v", empty, err)

				jsonReq, err := tsReq.MarshalJSON()
				if err != nil {
					log.Errorf("Failed to marshal time series as JSON")
					return
				}
				log.Errorf("JSON GCM: %s", string(jsonReq[:]))
				return
			}

			log.Errorf("Data has been written to GCM.")
		}()
		time.Sleep(resolution)
	}
}
