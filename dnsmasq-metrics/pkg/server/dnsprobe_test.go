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
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"

	"k8s.io/contrib/dnsmasq-metrics/pkg/test"
)

func makeOptions(label string, addr string, port int) *Options {
	return &Options{
		PrometheusNamespace: "test",
		Probes: []DNSProbeOption{
			{
				Label:    label,
				Server:   fmt.Sprintf("%v:%v", addr, port),
				Name:     "test.local.",
				Interval: 1 * time.Millisecond,
			},
		},
	}
}

func TestProbeOk(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)
	go server.Run(okResponseCallback)
	<-server.StartChan

	options := makeOptions("ok", addr, port)
	probe := &dnsProbe{DNSProbeOption: options.Probes[0]}
	probe.Start(options)

	// This is ugly and flaky but we wait for some probes to go through by sleeping.
	time.Sleep(250 * time.Millisecond)
	probe.lock.Lock()
	defer probe.lock.Unlock()

	if probe.lastError != nil {
		t.Errorf("should have no error: %v", probe.lastError)
	}
}

func TestProbeNx(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)
	go server.Run(nxResponseCallback)
	<-server.StartChan

	options := makeOptions("nx", addr, port)
	probe := &dnsProbe{DNSProbeOption: options.Probes[0]}
	probe.Start(options)

	time.Sleep(250 * time.Millisecond)
	probe.lock.Lock()
	defer probe.lock.Unlock()

	if probe.lastError == nil {
		t.Errorf("should have error")
	}
}

func TestProbeFail(t *testing.T) {
	server := &test.Server{}
	addr, port := server.Init(t)

	options := makeOptions("fail", addr, port)
	probe := &dnsProbe{DNSProbeOption: options.Probes[0]}
	probe.Start(options)

	time.Sleep(250 * time.Millisecond)
	probe.lock.Lock()
	defer probe.lock.Unlock()

	if probe.lastError == nil {
		t.Errorf("should have error")
	}
}

func okResponseCallback(server *test.Server, remoteAddr net.Addr, msg *dns.Msg) {
	bytes := makeResponsePacket(server.T, msg.Id, 1)
	_, err := server.Conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.T.Fatalf("error sending response: %v", err)
	}
}

func nxResponseCallback(server *test.Server, remoteAddr net.Addr, msg *dns.Msg) {
	bytes := makeResponsePacket(server.T, msg.Id, 0)
	_, err := server.Conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.T.Fatalf("error sending response: %v", err)
	}
}

func makeResponsePacket(t *testing.T, id uint16, responses int) []byte {
	answer, err := dns.NewRR("test.local. 100 IN A 1.2.3.4")
	if err != nil {
		t.Fatalf("dns.NewRR: %v", err)
	}

	msg := &dns.Msg{}
	msg.SetQuestion("test.local.", dns.TypeA)
	msg.Question[0].Qclass = dns.ClassANY

	for i := 0; i < responses; i++ {
		msg.Answer = append(msg.Answer, answer)
	}
	msg.Id = id

	buf, err := msg.Pack()
	if err != nil {
		t.Fatalf("msg.Pack(): %v", err)
	}

	return buf
}
