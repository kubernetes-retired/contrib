/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	kube_utils "k8s.io/contrib/cluster-autoscaler/utils/kubernetes"
	kube_api "k8s.io/kubernetes/pkg/api"
	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

var (
	flags = flag.NewFlagSet(
		`rescheduler: rescheduler --running-in-cluster=true`,
		flag.ExitOnError)

	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	housekeepingInterval = flags.Duration("housekeeping-interval", 10*time.Second,
		`How often rescheduler takes actions.`)

	systemNamespace = flags.String("system-namespace", kube_api.NamespaceSystem,
		`Namespace to watch for critical addons.`)
)

func main() {
	glog.Infof("Running Rescheduler")

	flags.Parse(os.Args)

	// Create kubeclient
	var kubeClient *kube_client.Client
	if *inCluster {
		var err error
		if kubeClient, err = kube_client.NewInCluster(); err != nil {
			glog.Fatalf("Failed to create client: %v.", err)
		}
	} else {
		clientConfig := kubectl_util.DefaultClientConfig(flags)
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient = kube_client.NewOrDie(config)
	}

	unschedulablePodLister := kube_utils.NewUnschedulablePodLister(kubeClient, *systemNamespace)

	for {
		select {
		case <-time.After(*housekeepingInterval):
			{
				allUnschedulablePods, err := unschedulablePodLister.List()
				if err != nil {
					glog.Errorf("Failed to list unscheduled pods: %v", err)
					continue
				}
				glog.Infof("%+v", allUnschedulablePods)
			}
		}
	}
}
