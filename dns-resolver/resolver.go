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
	"runtime"

	"github.com/golang/glog"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	confFile = "/etc/unbound/unbound.conf"
)

type forward struct {
	Name string
	IP   string
}

type resolverControl interface {
	FlushDomain(name string) error
	FlushHost(hostname string) error
}

type resolver struct {
	domain  string
	ns      []string
	forward []forward
}

func (r *resolver) Start() {
	if err := r.createConf(r.domain, r.ns, r.forward); err != nil {
		glog.Errorf("unbound error: %v", err)
	}

	cmd := exec.Command("unbound", "-c", confFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		glog.Errorf("unbound error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		glog.Fatalf("unbound error: %v", err)
	}
}

func (r resolver) IsHealthy() bool {
	cmd := exec.Command("unbound-control", "status")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (r resolver) FlushDomain(name string) error {
	glog.Infof("removing domain %v from unbound cache", name)
	cmd := exec.Command("unbound-control", "flush_zone", name)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (r resolver) FlushHost(hostname string) error {
	glog.Infof("removing host %v from unbound cache", hostname)
	cmd := exec.Command("unbound-control", "flush", hostname)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (r *resolver) Reload() error {
	glog.Info("reloading unbound...")
	_, err := k8sexec.New().Command("killall", "-HUP", "unbound").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading unbound: %v", err)
	}
	return nil
}

func (r *resolver) createConf(domain string, dnsServers []string, customForward []forward) error {
	conf := make(map[string]interface{})
	conf["domain"] = domain
	conf["dnsServers"] = dnsServers
	conf["customForward"] = customForward
	conf["cpus"] = runtime.NumCPU()
	return mergeTemplate("/unbound.tmpl", confFile, conf)
}
