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
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
	_ "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend/backends"
	"k8s.io/contrib/loadbalancer/loadbalancer-daemon/controllers"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var (
	flags     = pflag.NewFlagSet("", pflag.ExitOnError)
	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	watchNamespace = flag.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Configmap/Services/Endpoints. By default the controller
		watches acrosss all namespaces`)

	backendName = flags.String("backend", "nginx",
		`Backend to use. Default is nginx.`)

	runKeepalived = flags.Bool("keepalived", true,
		`Flag to determine whether keepalived manager is to be run. Default is true`)
)

func main() {

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	clientConfig := kubectl_util.DefaultClientConfig(flags)

	var kubeClient *client.Client

	var err error
	if *inCluster {
		kubeClient, err = client.NewInCluster()
	} else {
		config, connErr := clientConfig.ClientConfig()
		if connErr != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = client.New(config)
	}

	if err != nil {
		glog.Fatalf("failed to create client: %v", err)
	}

	backendController, err := factory.CreateBackendController(*backendName)
	if err != nil {
		glog.Fatalf("failed to create backend controller for %s: %v", *backendName, err)
	}
	configController, _ := controllers.NewConfigMapController(kubeClient, 30*time.Second, *watchNamespace, backendController, *runKeepalived)
	configController.Run()
}
