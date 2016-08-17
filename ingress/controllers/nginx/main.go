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
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/contrib/ingress/controllers/nginx/nginx"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/healthz"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	healthPort = 10249
)

var (
	// value overwritten during build. This can be used to resolve issues.
	version = "0.8.2"
	gitRepo = "https://github.com/kubernetes/contrib"

	flags = pflag.NewFlagSet("", pflag.ExitOnError)

	defaultSvc = flags.String("default-backend-service", "",
		`Service used to serve a 404 page for the default backend. Takes the form
    namespace/name. The controller uses the first node port of this Service for
    the default backend.`)

	nxgConfigMap = flags.String("nginx-configmap", "",
		`Name of the ConfigMap that containes the custom nginx configuration to use`)

	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	crossPermitted = flags.String("cross-permitted", "",
		`An optional comma seperate list of secrets which a ingress resource can request to serve upon,
		allowing you to store tls certificate in one namespace are reuse them across others. One possible
		use-case being sharing a wildcard certificate across the cluster. The format for this is
		namespace/secretName`)

	tcpConfigMapName = flags.String("tcp-services-configmap", "",
		`Name of the ConfigMap that containes the definition of the TCP services to expose.
		The key in the map indicates the external port to be used. The value is the name of the
		service with the format namespace/serviceName and the port of the service could be a number of the
		name of the port.
		The ports 80 and 443 are not allowed as external ports. This ports are reserved for nginx`)

	udpConfigMapName = flags.String("udp-services-configmap", "",
		`Name of the ConfigMap that containes the definition of the UDP services to expose.
		The key in the map indicates the external port to be used. The value is the name of the
		service with the format namespace/serviceName and the port of the service could be a number of the
		name of the port.`)

	resyncPeriod = flags.Duration("sync-period", 30*time.Second,
		`Relist and confirm cloud resources this often.`)

	watchNamespace = flags.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Ingress. Default is to watch all namespaces`)

	healthzPort = flags.Int("healthz-port", healthPort, "port for healthz endpoint.")

	buildCfg = flags.Bool("dump-nginx-configuration", false, `Returns a ConfigMap with the default nginx conguration.
		This can be used as a guide to create a custom configuration.`)

	profiling = flags.Bool("profiling", true, `Enable profiling via web interface host:port/debug/pprof/`)

	defSSLCertificate = flags.String("default-ssl-certificate", "", `Name of the secret that contains a SSL 
		certificate to be used as default for a HTTPS catch-all server`)
)

func main() {
	var kubeClient *unversioned.Client
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	clientConfig := kubectl_util.DefaultClientConfig(flags)

	glog.Infof("Using build: %v - %v", gitRepo, version)

	if *buildCfg {
		fmt.Printf("Example of ConfigMap to customize NGINX configuration:\n%v", nginx.ConfigMapAsString())
		os.Exit(0)
	}

	if *defaultSvc == "" {
		glog.Fatalf("Please specify --default-backend-service")
	}

	var crossPermittedTLS []string
	if *crossPermitted != "" {
		for _, x := range strings.Split(*crossPermitted, ",") {
			if len(strings.Split(x, "/")) != 2 {
				glog.Fatalf("the permitted tls pair %s is invalid, must be in format namespace/secretName", x)
			}
			crossPermittedTLS = append(crossPermittedTLS, x)
		}
	}

	var err error
	if *inCluster {
		kubeClient, err = unversioned.NewInCluster()
	} else {
		config, connErr := clientConfig.ClientConfig()
		if connErr != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = unversioned.New(config)
	}
	if err != nil {
		glog.Fatalf("failed to create client: %v", err)
	}

	runtimePodInfo := &podInfo{NodeIP: "127.0.0.1"}
	if *inCluster {
		runtimePodInfo, err = getPodDetails(kubeClient)
		if err != nil {
			glog.Fatalf("unexpected error getting runtime information: %v", err)
		}
	}
	if err := isValidService(kubeClient, *defaultSvc); err != nil {
		glog.Fatalf("no service with name %v found: %v", *defaultSvc, err)
	}
	glog.Infof("Validated %v as the default backend", *defaultSvc)

	if *nxgConfigMap != "" {
		_, _, err := parseNsName(*nxgConfigMap)
		if err != nil {
			glog.Fatalf("configmap error: %v", err)
		}
	}

	lbc, err := newLoadBalancerController(kubeClient, *resyncPeriod, *defaultSvc, *watchNamespace, *nxgConfigMap,
		*tcpConfigMapName, *udpConfigMapName, *defSSLCertificate, crossPermittedTLS, runtimePodInfo)
	if err != nil {
		glog.Fatalf("%v", err)
	}

	go registerHandlers(lbc)
	go handleSigterm(lbc)

	lbc.Run()

	for {
		glog.Infof("Handled quit, awaiting pod deletion")
		time.Sleep(30 * time.Second)
	}
}

// podInfo contains runtime information about the pod
type podInfo struct {
	PodName      string
	PodNamespace string
	NodeIP       string
}

func registerHandlers(lbc *loadBalancerController) {
	mux := http.NewServeMux()
	healthz.InstallHandler(mux, lbc.nginx)

	http.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "build: %v - %v", gitRepo, version)
	})

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		lbc.Stop()
	})

	if *profiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", *healthzPort),
		Handler: mux,
	}
	glog.Fatal(server.ListenAndServe())
}

func handleSigterm(lbc *loadBalancerController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	if err := lbc.Stop(); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}
