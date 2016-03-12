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
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/iptables"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	iptablesChain = "KEEPALIVED_VIP"
)

var (
	flags = pflag.NewFlagSet("", pflag.ContinueOnError)

	cluster = flags.Bool("use-kubernetes-cluster-service", true, `If true, use the built in kubernetes
        cluster for creating the client`)

	useUnicast = flags.Bool("use-unicast", false, `use unicast instead of multicast for communication
		with other keepalived instances`)

	// sysctl changes required by keepalived
	sysctlAdjustments = map[string]int{
		// allows processes to bind() to non-local IP addresses
		"net/ipv4/ip_nonlocal_bind": 1,
		// enable connection tracking for LVS connections
		"net/ipv4/vs/conntrack": 1,
	}
)

func main() {
	clientConfig := kubectl_util.DefaultClientConfig(flags)

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	var err error
	var kubeClient *unversioned.Client

	if *cluster {
		if kubeClient, err = unversioned.NewInCluster(); err != nil {
			glog.Fatalf("Failed to create client: %v", err)
		}
	} else {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = unversioned.New(config)
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
	}

	namespace, specified, err := clientConfig.Namespace()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	if !specified {
		namespace = api.NamespaceAll
	}

	err = loadIPVModule()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	err = changeSysctl()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	err = resetIPVS()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	ae, err := iptables.EnsureChain(iptables.TableNAT, iptables.Chain(iptablesChain))
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}
	if ae && glog.V(2) {
		glog.Infof("chain %v already existed", iptablesChain)
	}

	glog.Info("starting LVS configuration")
	if *useUnicast {
		glog.Info("keepalived will use unicast to sync the nodes")
	}
	ipvsc := newIPVSController(kubeClient, namespace, *useUnicast)
	go ipvsc.epController.Run(wait.NeverStop)
	go ipvsc.svcController.Run(wait.NeverStop)

	go wait.Until(ipvsc.sync, 10*time.Second, wait.NeverStop)

	time.Sleep(5 * time.Second)
	glog.Info("starting keepalived to announce VIPs")
	ipvsc.keepalived.Start()
}
