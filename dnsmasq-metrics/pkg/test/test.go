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

package test

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

// Server for mock DNS responses.
type Server struct {
	T         *testing.T
	Conn      *net.UDPConn
	StartChan chan struct{}
}

// ServerCallback to respond to DNS messages.
type ServerCallback func(server *Server, remoteAddr net.Addr, msg *dns.Msg)

// Init the DNS server.
func (s *Server) Init(t *testing.T) (addr string, port int) {
	s.T = t
	s.StartChan = make(chan struct{})

	var err error
	s.Conn, err = net.ListenUDP(
		"udp",
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 0, // allocate a free emphemeral port
		})
	if err != nil {
		panic(err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", s.Conn.LocalAddr().String())
	if err != nil {
		panic(err)
	}

	addr = localAddr.IP.String()
	port = localAddr.Port

	return
}

// Run the server with the given response callback.
func (s *Server) Run(cb ServerCallback) {
	close(s.StartChan)

	for {
		buf := make([]byte, 4096)
		len, remoteAddr, err := s.Conn.ReadFrom(buf)
		if err != nil {
			s.T.Fatalf("error reading packet: %v", err)
		}
		buf = buf[:len]

		msg := &dns.Msg{}
		err = msg.Unpack(buf)
		if err != nil {
			s.T.Fatalf("unable to Unpack %v: %v", buf, err)
		}

		cb(s, remoteAddr, msg)
	}
}
