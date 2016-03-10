/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/miekg/dns"
)

type nameserver interface {
	Dump() string
	Add(rr dns.RR)
	Remove(name string)
}

const (
	defTTL = 30
)

var _ nameserver = &inMemoryDNS{}

type inMemoryDNS struct {
	sync.Mutex
	server *dns.Server
	addr   net.Addr
	rr     map[string][]dns.RR
}

func makeNameserver(domain, clusterIP string) (nameserver, error) {
	ns := &inMemoryDNS{
		rr: make(map[string][]dns.RR),
	}
	if err := ns.startUDPServer(); err != nil {
		return nil, err
	}

	nsHost := fmt.Sprintf("ns.%v", domain)
	ns.Add(nsRecord(domain, nsHost, 0))
	ns.Add(aRecord(nsHost, clusterIP, 0))

	return ns, nil
}

func (ns *inMemoryDNS) startUDPServer() error {
	pc, err := net.ListenPacket("udp", "0.0.0.0:54")
	if err != nil {
		return err
	}
	ns.server = &dns.Server{
		PacketConn: pc,
		Handler:    ns,
	}
	ns.addr = pc.LocalAddr()
	go func() {
		ns.server.ActivateAndServe()
		pc.Close()
	}()
	return nil
}

func (ns *inMemoryDNS) lookup(q string) []dns.RR {
	r := strings.ToLower(q)
	glog.V(5).Infof("searching dns record for %v", r)
	ns.Lock()
	defer ns.Unlock()
	return ns.rr[r]
}

// ServeDNS custom dns.Handler
func (ns *inMemoryDNS) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(req)
	q := msg.Question[0].Name

	rr := ns.lookup(q)
	if len(rr) == 0 {
		w.Close()
		glog.V(2).Infof("no record found for: %v", q)
		return
	}

	glog.V(4).Infof("record found for: %v - %v", q, rr[0])
	msg.Answer = make([]dns.RR, 1)
	msg.Answer[0] = rr[0]
	w.WriteMsg(msg)
}

func (ns *inMemoryDNS) Add(rr dns.RR) {
	ns.Lock()
	defer ns.Unlock()
	glog.V(4).Infof("adding new record: %v", rr)
	ns.rr[rr.Header().Name] = append(ns.rr[rr.Header().Name], rr)
}

func (ns *inMemoryDNS) Remove(fqdn string) {
	ns.Lock()
	defer ns.Unlock()

	host := fmt.Sprintf("%v.", strings.ToLower(fqdn))
	if _, ok := ns.rr[host]; ok {
		glog.V(4).Infof("removing record: %v", fqdn)
		delete(ns.rr, fmt.Sprintf("%v", host))
	}
}

func (ns *inMemoryDNS) Dump() string {
	ns.Lock()
	defer ns.Unlock()

	b, err := json.MarshalIndent(ns.rr, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}

	return string(b)
}

func aRecord(fqdn, ip string, cttl int) dns.RR {
	host := strings.ToLower(fqdn)
	ttl := defTTL
	if cttl > 0 {
		ttl = cttl
	}

	// fqdn. ttl IN A ip address
	r, _ := dns.NewRR(fmt.Sprintf("%v. %v IN A %v", host, ttl, ip))
	return r
}

func srvRecord(fqdn, cname string, port, cttl int) dns.RR {
	host := strings.ToLower(fqdn)
	ttl := defTTL
	if cttl > 0 {
		ttl = cttl
	}

	// _service._proto.name. TTL class SRV priority weight port target.
	r, _ := dns.NewRR(fmt.Sprintf("%v. %v IN SRV 10 0 %v %v", host, ttl, port, cname))
	return r
}

func nsRecord(fqdn, ip string, cttl int) dns.RR {
	host := strings.ToLower(fqdn)
	ttl := defTTL
	if cttl > 0 {
		ttl = cttl
	}

	r, _ := dns.NewRR(fmt.Sprintf("%v. %v IN NS %v", host, ttl, ip))
	return r
}
