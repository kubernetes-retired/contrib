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
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	v3 "google.golang.org/api/monitoring/v3"
	"k8s.io/contrib/kubelet-to-gcm/monitor"
)

const (
	scope = "https://www.googleapis.com/auth/monitoring.write"
	//testPath = "https://test-monitoring.sandbox.googleapis.com"
)

var (
	// Flags to identify the Kubelet.
	zoneArg    = pflag.String("zone", "", "The zone where this kubelet lives.")
	projectArg = pflag.String("project", "", "The project where this kubelet's host lives.")
	cluster    = pflag.String("cluster", "unknown", "The cluster where this kubelet holds membership.")
	host       = pflag.String("host", "localhost", "The kubelet's host name.")
	port       = pflag.Uint("port", 10255, "The kubelet's port.")
	// Flags to control runtime behavior.
	resolution  = time.Second * time.Duration(*pflag.Uint("resolution", 10, "The time, in seconds, to poll the Kubelet."))
	gcmEndpoint = pflag.String("gcm-endpoint", "", "The GCM endpoint to hit. Defaults to the default endpoint.")
)

// getGCEMetaData hits the instance's MD server.
func getGCEMetaData(uri string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request %q for GCE metadata: %v", uri, err)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed request %q for GCE metadata: %v", uri, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read body for request %q for GCE metadata: %v", uri, err)
	}
	return body, nil
}

// getZone returns zone if it's set, and hits the GCE MD server otherwise.
func getZone(zone string) (string, error) {
	if zone == "" {
		body, err := getGCEMetaData("http://169.254.169.254/computeMetadata/v1/instance/zone")
		if err != nil {
			return "", fmt.Errorf("Failed to get zone from GCE: %v", err)
		}
		tokens := strings.Split(string(body), "/")
		if len(tokens) < 1 {
			return "", fmt.Errorf("Failed to parse GCE response %q for instance zone.", string(body))
		}
		zone = tokens[len(tokens)-1]
	}
	return zone, nil
}

// getProjectID returns projectID if it's set, and hits the GCE MD server otherwise.
func getProjectID(projectID string) (string, error) {
	if projectID == "" {
		body, err := getGCEMetaData("http://169.254.169.254/computeMetadata/v1/project/project-id")
		if err != nil {
			return "", fmt.Errorf("Failed to get zone from GCE: %v", err)
		}
		projectID = string(body)
	}
	return projectID, nil
}

func main() {
	// First log our starting config, and then set up.
	flag.Set("logtostderr", "true") // This spoofs glog into teeing logs to stderr.
	defer log.Flush()
	log.Infof("Invoked by %v", os.Args)
	pflag.Parse()

	// Determine what zone and project we're monitoring.
	zone, err := getZone(*zoneArg)
	if err != nil {
		log.Fatalf("Failed to get zone: %v", err)
	}
	project, err := getProjectID(*projectArg)
	if err != nil {
		log.Fatalf("Failed to get project: %v", err)
	}
	log.Infof("Using zone %q and project %q", zone, project)

	translator := monitor.NewTranslator(zone, project, *cluster, *host, resolution)
	log.Info("New Translator successfully created.")

	// NewKubeletClient validates its own inputs.
	kubelet, err := monitor.NewKubeletClient(*host, *port, nil)
	if err != nil {
		log.Fatalf("Failed to create a Kubelet client with host %s and port %d: %v", *host, *port, err)
	}
	log.Info("Successfully created kubelet client.")

	// Create a GCM client.
	name := fmt.Sprintf("projects/%s", project)
	client, err := google.DefaultClient(context.Background(), scope)
	if err != nil {
		log.Fatalf("Failed to create a client with default context and scope %s, err: %v", scope, err)
	}
	service, err := v3.New(client)
	if err != nil {
		log.Fatalf("Failed to create a GCM v3 API service object: %v", err)
	}
	// Determine the GCE endpoint.
	if *gcmEndpoint != "" {
		service.BasePath = *gcmEndpoint
	}
	log.Infof("Using GCM endpoint %q", service.BasePath)

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
		}()
		time.Sleep(resolution)
	}
}
