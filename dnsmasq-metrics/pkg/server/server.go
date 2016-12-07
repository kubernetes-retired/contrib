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
	"time"

	"github.com/golang/glog"
	"k8s.io/contrib/dnsmasq-metrics/pkg/dnsmasq"
)

// Server that runs the dnsmasq-metrics daemon.
type Server interface {
	Run(options *Options)
}

type server struct {
	options       *Options
	metricsClient dnsmasq.MetricsClient
	probes        []*dnsProbe
}

// NewServer creates a new server instance
func NewServer() Server {
	return &server{}
}

// Run the server (does not return)
func (s *server) Run(options *Options) {
	s.options = options
	glog.Infof("Starting server (options %+v)", *s.options)

	for _, probeOption := range options.Probes {
		probe := &dnsProbe{DNSProbeOption: probeOption}
		s.probes = append(s.probes, probe)
		probe.Start(options)
	}

	s.runMetrics(options)
}

func (s *server) runMetrics(options *Options) {
	InitializeMetrics(options)

	client := dnsmasq.NewMetricsClient(options.DnsMasqAddr, options.DnsMasqPort)

	for {
		metrics, err := client.GetMetrics()
		if err != nil {
			glog.Warningf("Error getting metrics from dnsmasq: %v", err)
			errorsCounter.Add(1)
		} else {
			glog.V(3).Infof("DnsMasq metrics %+v", metrics)
			exportMetrics(metrics)
		}

		time.Sleep(time.Duration(options.DnsMasqPollIntervalMs) * time.Millisecond)
	}
}

func exportMetrics(metrics *dnsmasq.Metrics) {
	for key := range *metrics {
		gauges[key].Set(float64((*metrics)[key]))
	}
}
