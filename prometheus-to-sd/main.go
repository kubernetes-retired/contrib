/*
Copyright 2017 The Kubernetes Authors.

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
	"time"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
	"k8s.io/contrib/prometheus-to-sd/translator"
)

var (
	host        = flag.String("target-host", "localhost", "The monitored target's hostname.")
	port        = flag.Uint("target-port", 80, "The monitored target's port.")
	component   = flag.String("component", "uknown", "The monitored target's name.")
	resolution  = flag.Duration("metrics-resolution", 60*time.Second, "The time, to poll the target.")
	prefix      = flag.String("stackdriver-prefix", "container.googleapis.com/master", "Prefix needs to be added to every metric.")
	whitelisted = flag.String("whitelisted-metrics", "", "Comma-separated list of whitelisted metrics. If empty all metrics will be exported.")
)

func main() {
	flag.Set("logtostderr", "true")
	defer glog.Flush()
	flag.Parse()

	glog.Infof("Running prometheus-to-sd, monitored target %s %v:%v", *component, *host, *port)

	gceConf, err := config.GetGceConfig(*prefix)
	if err != nil {
		glog.Fatalf("Failed to get GCE config: %v", err)
	}
	glog.Infof("GCE config: %+v", gceConf)

	client := oauth2.NewClient(oauth2.NoContext, google.ComputeTokenSource(""))
	stackdriverService, err := v3.New(client)
	if err != nil {
		glog.Fatalf("Failed to create Stackdriver client: %v", err)
	}
	glog.V(4).Infof("Successfully created Stackdriver client")

	for {
		glog.V(4).Infof("Scraping metrics")
		metrics, err := translator.GetPrometheusMetrics(*host, *port)
		if err != nil {
			glog.Warningf("Error while getting Prometheus metrics %v", err)
			continue
		}

		ts := translator.TranslatePrometheusToStackdriver(gceConf, *component, metrics, *whitelisted)
		translator.SendToStackdriver(stackdriverService, gceConf, ts)

		time.Sleep(*resolution)
	}
}
