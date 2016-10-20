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

package dnsmasq

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
	"github.com/miekg/dns"
)

// MetricsClient is a client used to obtain metrics from dnsmasq.
type MetricsClient interface {
	GetMetrics() (ret *Metrics, err error)
}

type metricsClient struct {
	addrPort  string
	dnsClient *dns.Client
}

// MetricName is a metric
type MetricName string

const (
	// CacheHits from dnsmasq.
	CacheHits MetricName = "hits"
	// CacheMisses from dnsmasq
	CacheMisses MetricName = "misses"
	// CacheEvictions from dnsmasq
	CacheEvictions MetricName = "evictions"
	// CacheInsertions from dnsmasq
	CacheInsertions MetricName = "insertions"
	// CacheSize from dnsmasq
	CacheSize MetricName = "cachesize"
)

// AllMetrics is a list of all exported metrics.
var AllMetrics = []MetricName{
	CacheHits, CacheMisses, CacheEvictions, CacheInsertions, CacheSize,
}

// Metrics exported by dnsmasq via *.bind CHAOS queries.
type Metrics map[MetricName]int64

// NewMetricsClient creates a new client for getting raw metrics from
// dnsmasq via the *.bind CHAOS TXT records. Note: this feature works
// for dnsmasq v2.76+, it is missing in older releases of the
// software.
func NewMetricsClient(addr string, port int) MetricsClient {
	return &metricsClient{
		addrPort:  fmt.Sprintf("%s:%d", addr, port),
		dnsClient: &dns.Client{},
	}
}

// GetMetrics requests metrics from dnsmasq and returns them.
func (mc *metricsClient) GetMetrics() (ret *Metrics, err error) {
	ret = &Metrics{}

	for i := range AllMetrics {
		var value int64
		metric := AllMetrics[i]
		value, err = mc.getSingleMetric(string(metric) + ".bind.")
		if err != nil {
			return
		}
		(*ret)[metric] = value
	}

	return
}

// Get a single metric from dnsmasq. Returns the numeric value of the
// metric.
func (mc *metricsClient) getSingleMetric(name string) (int64, error) {
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = false
	msg.Question = make([]dns.Question, 1)
	msg.Question[0] = dns.Question{
		Name:   name,
		Qtype:  dns.TypeTXT,
		Qclass: dns.ClassCHAOS,
	}

	in, _, err := mc.dnsClient.Exchange(msg, mc.addrPort)
	if err != nil {
		return 0, err
	}

	if len(in.Answer) != 1 {
		return 0, fmt.Errorf("Invalid number of Answer records for %s: %d",
			name, len(in.Answer))
	}

	if t, ok := in.Answer[0].(*dns.TXT); ok {
		glog.V(4).Infof("Got valid TXT response %+v for %s", t, name)
		if len(t.Txt) != 1 {
			return 0, fmt.Errorf("Invalid number of TXT records for %s: %d",
				name, len(t.Txt))
		}

		value, err := strconv.ParseInt(t.Txt[0], 10, 64)
		if err != nil {
			return 0, err
		}

		return value, nil
	}

	return 0, fmt.Errorf("missing txt record for %s", name)
}
