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
	"os"
	"time"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/client/unversioned"
	kapi "k8s.io/kubernetes/pkg/api"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/wait"
)

var (
	flags = flag.NewFlagSet("", flag.ContinueOnError)

	cluster = flags.Bool("use-kubernetes-cluster-service", true, `If true, use the built in kubernetes
        cluster for creating the client`)

	logLevel = flags.Int("v", 1, `verbose output`)

	useUnicast = flags.Bool("use-unicast", false, `use unicast instead of multicast for communication
		with other keepalived instances`)

	password = flags.String("vrrp-password", "", `If set it will use it as keepalived password instead of the
		generated one using information about the nodes`)

	enableIPVSConntrack = flags.Bool("enable-ipvs-conntrack", true,
		`enable ipvs conntrack in net/ipv4/vs/conntrack`)

	// sysctl changes required by keepalived
	sysctlAdjustments = map[string]int{
		// allows processes to bind() to non-local IP addresses
		"net/ipv4/ip_nonlocal_bind": 1,
	}

	// kernel modules to load and proc files to check
	kernelModules = map[string]string{
		// Linux ip virtual server
		"ip_vs": "/proc/net/ip_vs",
	}
)

func main() {
	clientConfig := kubectl_util.DefaultClientConfig(flags)
	flags.Parse(os.Args)

	if enableIPVSConntrack != nil && *enableIPVSConntrack {
		// enable connection tracking for LVS connections using nf_conntrack
		sysctlAdjustments["net/ipv4/vs/conntrack"] = 1
	}

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
	}

	namespace, specified, err := clientConfig.Namespace()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	if !specified {
		namespace = kapi.NamespaceAll
	}

	if !*debugKeeplivedConf {
		err = loadKernelModules()
		if err != nil {
			glog.Fatalf("Terminating execution: %v", err)
		}

		err = changeSysctl()
		if err != nil {
			glog.Fatalf("Terminating execution: %v", err)
		}
	}

	err = resetIPVS()
	if err != nil {
		glog.Fatalf("Terminating execution: %v", err)
	}

	glog.Info("starting LVS configuration")
	if *useUnicast {
		glog.Info("keepalived will use unicast to sync the nodes")
	}
	ipvsc := newIPVSController(kubeClient, namespace, *useUnicast, *password)
	go ipvsc.epController.Run(wait.NeverStop)
	go ipvsc.svcController.Run(wait.NeverStop)
	go wait.Until(ipvsc.worker, time.Second, wait.NeverStop)

	time.Sleep(5 * time.Second)

	ipvsc.keepalived.Start()
}
