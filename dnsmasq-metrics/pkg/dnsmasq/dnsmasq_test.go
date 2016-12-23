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
	"net"
	"reflect"
	"strings"
	"testing"

	"k8s.io/contrib/dnsmasq-metrics/pkg/test"

	"github.com/miekg/dns"
)

var expectedMetrics = Metrics{
	CacheHits:       10,
	CacheMisses:     20,
	CacheEvictions:  30,
	CacheInsertions: 40,
	CacheSize:       50,
}

func TestClientOk(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)
	client := NewMetricsClient(addr, port)

	go server.Run(validResponseCallback)
	<-server.StartChan

	metrics, err := client.GetMetrics()
	if err != nil {
		t.Errorf("Error in client.GetMetrics(): %v", err)
	}

	if !reflect.DeepEqual(*metrics, expectedMetrics) {
		t.Errorf("Expected %v, but got %v", expectedMetrics, metrics)
	}
}

func TestClientFail(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)
	client := NewMetricsClient(addr, port)

	go server.Run(junkResponseCallback)
	<-server.StartChan

	_, err := client.GetMetrics()
	if err == nil {
		t.Errorf("Expected err, got nil")
	}
}

func TestClientTimeout(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)
	client := NewMetricsClient(addr, port)

	_, err := client.GetMetrics()
	if err == nil {
		t.Errorf("Expected err, got nil")
	}
}

// validResponseCallback responds with the expectedMetrics.
func validResponseCallback(server *test.Server, remoteAddr net.Addr, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		server.T.Fatalf("invalid number of question entries: %v", msg.Question)
	}

	name := msg.Question[0].Name
	const suffix = ".bind."
	if !strings.HasSuffix(name, suffix) {
		server.T.Fatalf("invalid DNS suffix: %v", name)
	}

	metric := MetricName(name[:len(name)-len(suffix)])

	found := false
	for i := range AllMetrics {
		if metric == AllMetrics[i] {
			found = true
		}
	}
	if !found {
		server.T.Fatalf("invalid metric: %v", msg.Question)
	}

	val := expectedMetrics[metric]
	bytes := makeResponsePacket(server.T, msg.Id, name, val)
	_, err := server.Conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.T.Fatalf("error sending response: %v", err)
	}
}

// junkResponseCallback responds with an invalid packet.
func junkResponseCallback(server *test.Server, remoteAddr net.Addr, msg *dns.Msg) {
	bytes := []byte("junk")
	_, err := server.Conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.T.Fatalf("error sending response: %v", err)
	}
}

// Create a valid response packet for <name>.bind.
func makeResponsePacket(t *testing.T, id uint16, name string, value int64) []byte {
	answer, err := dns.NewRR(
		name + " 100 CH TXT " + fmt.Sprintf("%d", value))
	if err != nil {
		t.Fatalf("dns.NewRR: %v", err)
	}
	msg := &dns.Msg{}
	msg.SetQuestion(name, dns.TypeTXT)
	msg.Question[0].Qclass = dns.ClassCHAOS
	msg.Answer = append(msg.Answer, answer)
	msg.Id = id

	buf, err := msg.Pack()
	if err != nil {
		t.Fatalf("msg.Pack(): %v", err)
	}

	return buf
}
