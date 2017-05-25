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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/contrib/fluentd/event-exporter/sinks"
	"k8s.io/contrib/fluentd/event-exporter/sinks/stackdriver"
)

var (
	resyncPeriod = flag.Duration("resync-period", 1*time.Minute, "Reflector resync period")
	sinkType     = flag.String("sink-type", stackdriver.SdSinkName, "Name of the sink "+
		"used for events ingestion")
	sinkOpts           = flag.String("sink-opts", "", "Parameters for configuring sink")
	prometheusEndpoint = flag.String("prometheus-endpoint", ":80", "Endpoint on which to "+
		"expose Prometheus http handler")
)

func newSystemStopChannel() chan struct{} {
	ch := make(chan struct{})
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		glog.Infof("Recieved signal %s, terminating", sig.String())

		ch <- struct{}{}
	}()

	return ch
}

func newKubernetesClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	return kubernetes.NewForConfig(config)
}

func newSink() (sinks.Sink, error) {
	sinkFactory, ok := sinks.KnownSinks[*sinkType]
	if !ok {
		options := []string{}
		for k := range sinks.KnownSinks {
			options = append(options, k)
		}
		optionsStr := strings.Join(options, ", ")
		return nil, fmt.Errorf("unknown sink type: %s. Should be one of: %s", *sinkType, optionsStr)
	}
	return sinkFactory.CreateNew(strings.Split(*sinkOpts, " "))
}

func main() {
	flag.Set("logtostderr", "true")
	defer glog.Flush()
	flag.Parse()

	sink, err := newSink()
	if err != nil {
		glog.Fatalf("Failed to initialize sink: %v", err)
	}
	client, err := newKubernetesClient()
	if err != nil {
		glog.Fatalf("Failed to initialize kubernetes client: %v", err)
	}

	eventExporter := newEventExporter(client, sink, *resyncPeriod)

	// Expose the Prometheus http endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		glog.Fatalf("Prometheus monitoring failed: %v", http.ListenAndServe(*prometheusEndpoint, nil))
	}()

	stopCh := newSystemStopChannel()
	eventExporter.Run(stopCh)
}
