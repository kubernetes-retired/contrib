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
	udpConn, addr, port := setupConn()
	client := NewMetricsClient(addr, port)

	server := &server{t, udpConn, make(chan struct{})}
	go server.run(validResponseCallback)
	<-server.startChan

	metrics, err := client.GetMetrics()
	if err != nil {
		t.Errorf("Error in client.GetMetrics(): %v", err)
	}

	if !reflect.DeepEqual(*metrics, expectedMetrics) {
		t.Errorf("Expected %v, but got %v", expectedMetrics, metrics)
	}
}

func TestClientFail(t *testing.T) {
	udpConn, addr, port := setupConn()
	client := NewMetricsClient(addr, port)

	server := &server{t, udpConn, make(chan struct{})}
	go server.run(junkResponseCallback)
	<-server.startChan

	_, err := client.GetMetrics()
	if err == nil {
		t.Errorf("Expected err, got nil")
	}
}

func TestClientTimeout(t *testing.T) {
	_, addr, port := setupConn()
	client := NewMetricsClient(addr, port)

	_, err := client.GetMetrics()
	if err == nil {
		t.Errorf("Expected err, got nil")
	}
}

func setupConn() (server *net.UDPConn, addr string, port int) {
	var err error
	server, err = net.ListenUDP(
		"udp",
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 0, // allocate a free emphemeral port
		})
	if err != nil {
		panic(err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", server.LocalAddr().String())
	if err != nil {
		panic(err)
	}

	addr = localAddr.IP.String()
	port = localAddr.Port
	return
}

type ServerCallback func(server *server, remoteAddr net.Addr, msg *dns.Msg)

type server struct {
	t         *testing.T
	conn      *net.UDPConn
	startChan chan struct{}
}

func (server *server) run(cb ServerCallback) {
	close(server.startChan)

	for {
		buf := make([]byte, 4096)
		len, remoteAddr, err := server.conn.ReadFrom(buf)
		if err != nil {
			server.t.Fatalf("error reading packet: %v", err)
		}
		buf = buf[:len]

		msg := &dns.Msg{}
		err = msg.Unpack(buf)
		if err != nil {
			server.t.Fatalf("unable to Unpack %v: %v", buf, err)
		}

		cb(server, remoteAddr, msg)
	}
}

// validResponseCallback responds with the expectedMetrics.
func validResponseCallback(server *server, remoteAddr net.Addr, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		server.t.Fatalf("invalid number of question entries: %v", msg.Question)
	}

	name := msg.Question[0].Name
	const suffix = ".bind."
	if !strings.HasSuffix(name, suffix) {
		server.t.Fatalf("invalid DNS suffix: %v", name)
	}

	metric := MetricName(name[:len(name)-len(suffix)])

	found := false
	for i := range AllMetrics {
		if metric == AllMetrics[i] {
			found = true
		}
	}
	if !found {
		server.t.Fatalf("invalid metric: %v", msg.Question)
	}

	val := expectedMetrics[metric]
	bytes := makeResponsePacket(server.t, msg.Id, name, val)
	_, err := server.conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.t.Fatalf("error sending response: %v", err)
	}
}

// junkResponseCallback responds with an invalid packet.
func junkResponseCallback(server *server, remoteAddr net.Addr, msg *dns.Msg) {
	bytes := []byte("junk")
	_, err := server.conn.WriteTo(bytes, remoteAddr)
	if err != nil {
		server.t.Fatalf("error sending response: %v", err)
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
