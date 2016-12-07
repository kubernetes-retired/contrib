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
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

type dnsProbe struct {
	DNSProbeOption

	lock               sync.Mutex
	lastResolveLatency time.Duration
	lastError          error
	latencyHistogram   prometheus.Histogram
	errorCount         prometheus.Counter
}

func (p *dnsProbe) Start(options *Options) {
	glog.V(2).Infof("Starting dnsProbe %+v", p.DNSProbeOption)

	p.lastError = fmt.Errorf("waiting for first probe")

	http.HandleFunc("/healthcheck/"+p.Label, p.httpHandler)
	p.registerMetrics(options)

	go p.loop()
}

func (p *dnsProbe) registerMetrics(options *Options) {
	const dnsProbeSubsystem = "probe"

	p.latencyHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: options.PrometheusNamespace,
		Subsystem: dnsProbeSubsystem,
		Name:      p.Label + "_latency_ms",
		Help:      "Latency of the DNS probe request " + p.Label,
		Buckets:   prometheus.LinearBuckets(0, 10, 500),
	})
	prometheus.MustRegister(p.latencyHistogram)

	p.errorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: options.PrometheusNamespace,
		Subsystem: dnsProbeSubsystem,
		Name:      p.Label + "_errors",
		Help:      "Count of errors in name resolution of " + p.Label,
	})
	prometheus.MustRegister(p.errorCount)
}

func (p *dnsProbe) loop() {
	// Stagger the start of the loop to avoid sending all probes at exactly the same time.
	time.Sleep(time.Duration(rand.Int63n((int64)(p.Interval))))

	dnsClient := &dns.Client{}

	for {
		glog.V(4).Infof("Sending DNS request @%v %v", p.Server, p.Name)
		msg, latency, err := dnsClient.Exchange(p.msg(), p.Server)
		glog.V(4).Infof("Got response, err=%v after %v", err, latency)

		if err == nil && len(msg.Answer) == 0 {
			err = fmt.Errorf("no RRs for domain %q", p.Name)
		}

		p.lock.Lock()
		if err == nil {
			p.lastResolveLatency = latency
			p.lastError = nil

			p.latencyHistogram.Observe(latency.Seconds() * 1000)
		} else {
			glog.V(3).Infof("DNS resolution error for %v: %v", p.Label, err)
			p.lastResolveLatency = 0
			p.lastError = err

			p.errorCount.Add(1)
		}
		p.lock.Unlock()

		sleepInterval := p.Interval - latency
		if sleepInterval > 0 {
			glog.V(4).Infof("Sleeping %v", sleepInterval)
			time.Sleep(sleepInterval)
		}
	}
}

func (p *dnsProbe) msg() (msg *dns.Msg) {
	msg = new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)
	msg.Question[0] = dns.Question{
		Name:   p.Name,
		Qtype:  dns.TypeANY,
		Qclass: dns.ClassINET,
	}
	return
}

func (p *dnsProbe) httpHandler(w http.ResponseWriter, r *http.Request) {
	p.lock.Lock()
	defer p.lock.Unlock()

	response := struct {
		IsOk           bool
		LatencySeconds float64
		Err            string
	}{}

	if p.lastError == nil {
		response.IsOk = true
		response.LatencySeconds = p.lastResolveLatency.Seconds()

		w.Header().Add("Content-Type", "application/json")
		if buf, err := json.Marshal(response); err == nil {
			w.WriteHeader(200)
			w.Write(buf)
		} else {
			w.WriteHeader(500)
			glog.Errorf("JSON Marshal error: %v", err)
		}
	} else {
		response.IsOk = false
		response.Err = p.lastError.Error()

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(500)

		if buf, err := json.Marshal(response); err == nil {
			w.Write(buf)
		} else {
			glog.Errorf("JSON Marshal error: %v", err)
		}
	}
}
