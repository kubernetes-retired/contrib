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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/golang/glog"
)

// Entrypoint of GLBC. Example invocation:
// 1. In a pod:
// glbc --delete-all-on-quit
// 2. Dry run (on localhost):
// $ kubectl proxy --api-prefix="/"
// $ glbc --proxy="http://localhost:proxyport"

const (
	// lbApiPort is the port on which the loadbalancer controller serves a
	// minimal api (/healthz, /delete-all-and-quit etc).
	lbApiPort = 8081
)

var (
	flags = flag.NewFlagSet(
		`gclb: gclb --runngin-in-cluster=false --default-backend-node-port=123`,
		flag.ExitOnError)

	proxyUrl = flags.String("proxy", "",
		`If specified, the controller assumes a kubctl proxy server is running on the
		given url and creates a proxy client and fake cluster manager. Results are
		printed to stdout and no changes are made to your cluster. This flag is for
		testing.`)

	clusterName = flags.String("gce-cluster-name", "default-cluster-name",
		`Optional, used to tag cluster wide, shared loadbalancer resources such
		 as instance groups. Use this flag if you'd like to continue using the
		 same resources across a pod restart. Note that this does not need to
		 match the name of you Kubernetes cluster, it's just an arbitrary name
		 used to tag/lookup cloud resources.`)

	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	resyncPeriod = flags.Duration("sync-period", 30*time.Second,
		`Relist and confirm cloud resources this often.`)

	deleteAllOnQuit = flags.Bool("delete-all-on-quit", false,
		`If true, the controller will delete all Ingress and the associated
		external cloud resources as it's shutting down. Mostly used for
		testing. In normal environments the controller should only delete
		a loadbalancer if the associated Ingress is deleted.`)

	defaultSvc = flags.String("default-backend-service", "kube-system/default-http-backend",
		`Service used to serve a 404 page for the default backend. Takes the form
		namespace/name. The controller uses the first node port of this Service for
		the default backend.`)

	healthCheckPath = flags.String("health-check-path", "/",
		`Path used to health-check a backend service. All Services must serve
		a 200 page on this path. Currently this is only configurable globally.`)
)

func registerHandlers(lbc *loadBalancerController) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := lbc.clusterManager.isHealthy(); err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("Cluster unhealthy: %v", err)))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/delete-all-and-quit", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Retry failures during shutdown.
		lbc.Stop(true)
	})

	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", lbApiPort), nil))
}

func handleSigterm(lbc *loadBalancerController, deleteAll bool) {
	// Multiple SIGTERMs will get dropped
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Received SIGTERM, shutting down")

	// TODO: Better retires than relying on restartPolicy.
	exitCode := 0
	if err := lbc.Stop(deleteAll); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}
	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}

// main function for GLBC.
func main() {
	// TODO: Add a healthz endpoint
	var kubeClient *client.Client
	var err error
	var clusterManager *ClusterManager
	flags.Parse(os.Args)
	clientConfig := kubectl_util.DefaultClientConfig(flags)

	if *defaultSvc == "" {
		glog.Fatalf("Please specify --default-backend")
	}

	if *proxyUrl != "" {
		// Create proxy kubeclient
		kubeClient = client.NewOrDie(&client.Config{
			Host: *proxyUrl, Version: "v1"})
	} else {
		// Create kubeclient
		if *inCluster {
			if kubeClient, err = client.NewInCluster(); err != nil {
				glog.Fatalf("Failed to create client: %v.", err)
			}
		} else {
			config, err := clientConfig.ClientConfig()
			if err != nil {
				glog.Fatalf("error connecting to the client: %v", err)
			}
			kubeClient, err = client.New(config)
		}
	}
	// Wait for the default backend Service. There's no pretty way to do this.
	parts := strings.Split(*defaultSvc, "/")
	if len(parts) != 2 {
		glog.Fatalf("Default backend should take the form namespace/name: %v",
			*defaultSvc)
	}
	defaultBackendNodePort, err := getNodePort(kubeClient, parts[0], parts[1])
	if err != nil {
		glog.Fatalf("Could not configure default backend %v: %v",
			*defaultSvc, err)
	}

	if *proxyUrl == "" && *inCluster {
		// Create cluster manager
		clusterManager, err = NewClusterManager(
			*clusterName, defaultBackendNodePort, *healthCheckPath)
		if err != nil {
			glog.Fatalf("%v", err)
		}
	} else {
		// Create fake cluster manager
		clusterManager = newFakeClusterManager(*clusterName).ClusterManager
	}

	// Start loadbalancer controller
	lbc, err := NewLoadBalancerController(kubeClient, clusterManager, *resyncPeriod)
	if err != nil {
		glog.Fatalf("%v", err)
	}
	glog.Infof("Created lbc %+v", clusterManager.ClusterName)
	go registerHandlers(lbc)
	go handleSigterm(lbc, *deleteAllOnQuit)

	lbc.Run()
	for {
		glog.Infof("Handled quit, awaiting pod deletion.")
		time.Sleep(30 * time.Second)
	}
}
