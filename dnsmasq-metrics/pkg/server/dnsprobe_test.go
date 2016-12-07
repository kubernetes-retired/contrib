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

type mockLoopDelayer struct {
	sleepDone chan struct{}
}

func (d *mockLoopDelayer) Start(interval time.Duration) {
}

func (d *mockLoopDelayer) Sleep(latency time.Duration) {
	d.sleepDone <- struct{}{}
}

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
	testProbe(t, "ok", false, okResponseCallback)
}

func TestProbeNx(t *testing.T) {
	testProbe(t, "nx", true, nxResponseCallback)
}

func TestProbeFail(t *testing.T) {
	testProbe(t, "fail", true, nil)
}

func testProbe(t *testing.T, name string, hasError bool, callback test.ServerCallback) {
	server := &test.Server{}
	addr, port := server.Init(t)

	if callback != nil {
		go server.Run(callback)
	}

	// Note: sleepDone MUST be created here, otherwise there is a race
	// creating the channel.
	delayer := &mockLoopDelayer{sleepDone: make(chan struct{})}
	options := makeOptions(name, addr, port)
	probe := &dnsProbe{
		DNSProbeOption: options.Probes[0],
		delayer:        delayer,
	}

	probe.Start(options)

	// Wait for one loop to have been completed.
	<-delayer.sleepDone

	probe.lock.Lock()
	defer probe.lock.Unlock()

	if hasError {
		if probe.lastError == nil {
			t.Errorf("should have error")
		}
	} else {
		if probe.lastError != nil {
			t.Errorf("should have no error: %v", probe.lastError)
		}
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
