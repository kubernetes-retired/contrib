/*
Copyright 2015 The Kubernetes Authors.

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

	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/controller"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/version"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/healthz"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func main() {
	const (
		healthPort = 10254
	)

	var (
		flags = pflag.NewFlagSet("", pflag.ExitOnError)

		defaultSvc = flags.String("default-backend-service", "",
			`Service used to serve a 404 page for the default backend. Takes the form
    	namespace/name. The controller uses the first node port of this Service for
    	the default backend.`)

		nxgConfigMap = flags.String("nginx-configmap", "",
			`Name of the ConfigMap that containes the custom nginx configuration to use`)

		tcpConfigMapName = flags.String("tcp-services-configmap", "",
			`Name of the ConfigMap that containes the definition of the TCP services to expose.
		The key in the map indicates the external port to be used. The value is the name of the
		service with the format namespace/serviceName and the port of the service could be a 
		number of the name of the port.
		The ports 80 and 443 are not allowed as external ports. This ports are reserved for nginx`)

		udpConfigMapName = flags.String("udp-services-configmap", "",
			`Name of the ConfigMap that containes the definition of the UDP services to expose.
		The key in the map indicates the external port to be used. The value is the name of the
		service with the format namespace/serviceName and the port of the service could be a 
		number of the name of the port.`)

		resyncPeriod = flags.Duration("sync-period", 30*time.Second,
			`Relist and confirm cloud resources this often.`)

		watchNamespace = flags.String("watch-namespace", api.NamespaceAll,
			`Namespace to watch for Ingress. Default is to watch all namespaces`)

		healthzPort = flags.Int("healthz-port", healthPort, "port for healthz endpoint.")

		profiling = flags.Bool("profiling", true, `Enable profiling via web interface host:port/debug/pprof/`)

		defSSLCertificate = flags.String("default-ssl-certificate", "", `Name of the secret 
		that contains a SSL certificate to be used as default for a HTTPS catch-all server`)

		defHealthzURL = flags.String("health-check-path", "/ingress-controller-healthz", `Defines 
		the URL to be used as health check inside in the default server in NGINX.`)
	)

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	clientConfig := kubectl_util.DefaultClientConfig(flags)

	glog.Infof("Using build version %v from repo %v commit %v", version.RELEASE, version.REPO, version.BUILD)

	if *defaultSvc == "" {
		glog.Fatalf("Please specify --default-backend-service")
	}

	kubeClient, err := unversioned.NewInCluster()
	if err != nil {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error configuring the client: %v", err)
		}
		kubeClient, err = unversioned.New(config)
		if err != nil {
			glog.Fatalf("failed to create client: %v", err)
		}
	}

	if err := isValidService(kubeClient, *defaultSvc); err != nil {
		glog.Fatalf("no service with name %v found: %v", *defaultSvc, err)
	}
	glog.Infof("Validated %v as the default backend", *defaultSvc)

	if *nxgConfigMap != "" {
		_, _, err = parseNsName(*nxgConfigMap)
		if err != nil {
			glog.Fatalf("configmap error: %v", err)
		}
	}

	config := &controller.Configuration{
		Client:                kubeClient,
		ResyncPeriod:          *resyncPeriod,
		DefaultService:        *defaultSvc,
		Namespace:             *watchNamespace,
		NginxConfigMapName:    *nxgConfigMap,
		TCPConfigMapName:      *tcpConfigMapName,
		UDPConfigMapName:      *udpConfigMapName,
		DefaultSSLCertificate: *defSSLCertificate,
		DefaultHealthzURL:     *defHealthzURL,
		LeaderElection:        leaderelection.DefaultLeaderElectionConfiguration(),
	}

	leaderelection.BindFlags(&config.LeaderElection, flags)

	ic, err := controller.NewLoadBalancer(config)
	if err != nil {
		glog.Fatalf("%v", err)
	}

	go registerHandlers(*profiling, *healthzPort, ic)
	go handleSigterm(ic)

	ic.Start()

	for {
		glog.Infof("Handled quit, awaiting pod deletion")
		time.Sleep(30 * time.Second)
	}
}

func registerHandlers(enableProfiling bool, port int, ic controller.IngressController) {
	mux := http.NewServeMux()
	healthz.InstallHandler(mux, ic.Check())

	mux.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "build version %v from repo %v commit %v", version.RELEASE, version.REPO, version.BUILD)
	})

	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		ic.Stop()
	})

	if enableProfiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}
	glog.Fatal(server.ListenAndServe())
}

func handleSigterm(ic controller.IngressController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("The process received the a signal (SIGTERM), shutting down...")
	exitCode := 0
	if err := ic.Stop(); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}

func isValidService(kubeClient *unversioned.Client, name string) error {
	if name == "" {
		return fmt.Errorf("empty string is not a valid service name")
	}

	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid name format (namespace/name) in service '%v'", name)
	}

	_, err := kubeClient.Services(parts[0]).Get(parts[1])
	return err
}

func parseNsName(input string) (string, string, error) {
	nsName := strings.Split(input, "/")
	if len(nsName) != 2 {
		return "", "", fmt.Errorf("invalid format (namespace/name) found in '%v'", input)
	}

	return nsName[0], nsName[1], nil
}
