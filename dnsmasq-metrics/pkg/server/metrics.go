/*
Copyright 2016 The Kubernetes Authors.

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

package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/contrib/dnsmasq-metrics/pkg/dnsmasq"
)

var (
	gauges = make(map[dnsmasq.MetricName]prometheus.Gauge)

	errorsCounter prometheus.Counter
)

func defineDnsmasqMetrics(options *Options) {
	const dnsmasqSubsystem = "dnsmasq"

	gauges[dnsmasq.CacheHits] = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "hits",
			Help:      "Number of DNS cache hits (from start of process)",
		})
	gauges[dnsmasq.CacheMisses] = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "misses",
			Help:      "Number of DNS cache misses (from start of process)",
		})
	gauges[dnsmasq.CacheEvictions] = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "evictions",
			Help:      "Counter of DNS cache evictions (from start of process)",
		})
	gauges[dnsmasq.CacheInsertions] = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "insertions",
			Help:      "Counter of DNS cache insertions (from start of process)",
		})
	gauges[dnsmasq.CacheSize] = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "max_size",
			Help:      "Maximum size of the DNS cache",
		})

	for i := range gauges {
		prometheus.MustRegister(gauges[i])
	}

	errorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: options.PrometheusNamespace,
			Subsystem: dnsmasqSubsystem,
			Name:      "errors",
			Help:      "Number of errors that have occurred getting metrics",
		})
	prometheus.MustRegister(errorsCounter)
}

// InitializeMetrics and export metrics.
func InitializeMetrics(options *Options) {
	defineDnsmasqMetrics(options)

	http.Handle(options.PrometheusPath, prometheus.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "ok (%v)\n", time.Now())
	})

	go func() {
		err := http.ListenAndServe(
			fmt.Sprintf("%s:%d", options.PrometheusAddr, options.PrometheusPort), nil)
		if err != nil {
			glog.Fatalf("Error starting metrics server: %v", err)
		}
	}()
}
