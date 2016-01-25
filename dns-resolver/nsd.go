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
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/golang/glog"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	nsdConf = "/etc/nsd/nsd.conf"
	zone    = "/etc/nsd/kubernetes.zone"
	reverse = "/etc/nsd/reverse.zone"
	minTTL  = 30
)

type soa struct {
	Serial  int
	Refresh int
	Retry   int
	Expire  int
	MinTTL  int
}

type a struct {
	Host    string
	IP      string
	Reverse string
	UsePtr  bool
}

type cname struct {
	Host   string
	Target string
}

type srv struct {
	Host     string
	Priority int
	Weight   int
	Port     int
	Target   string
}

type dnsDomain struct {
	name  string
	soa   soa
	ns    a
	a     []a
	srv   []srv
	cname []cname
	mlock *sync.RWMutex
}

func newNsdBackend(domain, ip string) nsd {
	soa := soa{
		Serial:  newSerial(),
		Refresh: 28800,
		Retry:   7200,
		Expire:  604800,
		MinTTL:  minTTL,
	}

	nsFqdn := fmt.Sprintf("ns1.%v", domain)

	backend := nsd{
		data: &dnsDomain{
			name:  domain,
			soa:   soa,
			ns:    a{nsFqdn, ip, buildPtrDomain(ip), true},
			mlock: &sync.RWMutex{},
		},
	}

	backend.AddHost(nsFqdn, ip)

	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				newSha := checksum([]string{zone, reverse})
				if newSha != backend.sha {
					backend.Reload()
					backend.sha = newSha
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return backend
}

type nsd struct {
	data *dnsDomain
	sha  string
}

func (k nsd) IsHealthy() bool {
	cmd := exec.Command("nsd-control", "status")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func (k nsd) AddHost(host, ip string) error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	aRecord := a{
		Host:    host,
		IP:      ip,
		Reverse: buildPtr(ip),
	}

	if k.data.ns.Reverse == buildPtrDomain(ip) {
		aRecord.UsePtr = true
	}

	k.data.a = append(k.data.a, aRecord)

	return nil
}

func (k nsd) AddCname(alias, target string) error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	k.data.cname = append(k.data.cname, cname{
		Host:   alias,
		Target: target,
	})

	return nil
}

func (k nsd) RemoveCname(alias string) error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	for i := len(k.data.cname) - 1; i >= 0; i-- {
		curHost := k.data.cname[i]
		haveToDelete := false
		if curHost.Target == alias {
			haveToDelete = true
		}

		if haveToDelete {
			k.data.cname = append(k.data.cname[:i], k.data.cname[i+1:]...)
		}
	}

	return nil
}

func (k nsd) AddSrv(fqdn, target string, port int) error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	k.data.srv = append(k.data.srv, srv{
		Host:   fqdn,
		Port:   port,
		Target: target,
		Weight: 10,
	})

	return nil
}

func (k nsd) RemoveHost(host, ip string) error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	for i := len(k.data.a) - 1; i >= 0; i-- {
		curHost := k.data.a[i]
		haveToDelete := false
		if curHost.Host == host {
			haveToDelete = true
		}

		if haveToDelete {
			k.data.a = append(k.data.a[:i], k.data.a[i+1:]...)
		}
	}

	for i := len(k.data.cname) - 1; i >= 0; i-- {
		curHost := k.data.cname[i]
		haveToDelete := false
		if curHost.Target == host {
			haveToDelete = true
		}

		if haveToDelete {
			k.data.cname = append(k.data.cname[:i], k.data.cname[i+1:]...)
		}
	}

	return nil
}

func (k nsd) Remove(fqdn string) error {
	return k.RemoveHost(fqdn, "")
}

func (k nsd) writeCfg() error {
	k.data.mlock.Lock()
	defer k.data.mlock.Unlock()

	conf := make(map[string]interface{})
	conf["domain"] = k.data.name
	conf["soa"] = k.data.soa
	conf["ns"] = k.data.ns
	conf["a"] = k.data.a
	conf["srv"] = k.data.srv
	conf["cname"] = k.data.cname
	conf["ptrDomain"] = buildPtrDomain(k.data.ns.IP)
	conf["defTTL"] = minTTL

	err := mergeTemplate("/nsd.tmpl", nsdConf, conf)
	if err != nil {
		return err
	}

	err = mergeTemplate("/zone.tmpl", zone, conf)
	if err != nil {
		return err
	}

	err = mergeTemplate("/reverse.tmpl", reverse, conf)
	if err != nil {
		return err
	}

	return nil
}

func newSerial() int {
	t := time.Now()
	ts, _ := fmt.Printf("%d%02d%d%02d", t.Year(), t.Day(), t.Month(), 1)
	return ts
}

// Start starts nsd server in foreground
func (k nsd) Start() {
	glog.Info("starting nsd server")

	os.MkdirAll("/etc/nsd/", 0644)
	cmd := exec.Command("nsd-control-setup")
	if err := cmd.Run(); err != nil {
		glog.Fatalf("nsd error: %v", err)
	}

	if err := k.writeCfg(); err != nil {
		glog.Fatalf("nsd error: %v", err)
	}

	cmd = exec.Command("/usr/sbin/nsd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		glog.Errorf("nsd error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		glog.Fatalf("nsd error: %v", err)
	}
}

// Reload reloads nsd.conf and apply changes to TSIG keys and configuration
// patterns, add and remove zones that are mentioned in the config
func (k nsd) Reload() error {
	glog.Info("reloading nsd server")
	if err := k.writeCfg(); err != nil {
		glog.Errorf("nsd error: %v", err)
	}

	_, err := k8sexec.New().Command("nsd-control", "reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading nsd: %v", err)
	}

	return nil
}
