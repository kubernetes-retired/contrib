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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmd_api "k8s.io/client-go/tools/clientcmd/api"
)

var (
	flags = pflag.NewFlagSet("", pflag.ContinueOnError)

	apiserverHost = flags.String("apiserver-host", "", "The address of the Kubernetes Apiserver "+
		"to connect to in the format of protocol://address:port, e.g., "+
		"http://localhost:8080. If not specified, the assumption is that the binary runs inside a "+
		"Kubernetes cluster and local discovery is attempted.")

	kubeConfigFile = flags.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")

	watchNamespace = flags.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Ingress. Default is to watch all namespaces`)

	useUnicast = flags.Bool("use-unicast", false, `use unicast instead of multicast for communication
		with other keepalived instances`)

	configMapName = flags.String("services-configmap", "",
		`Name of the ConfigMap that contains the definition of the services to expose.
		The key in the map indicates the external IP to use. The value is the name of the 
		service with the format namespace/serviceName and the port of the service could be a number or the
		name of the port.`)

	proxyMode = flags.Bool("proxy-protocol-mode", false, `If true, it will use keepalived to announce the virtual
		IP address/es and HAProxy with proxy protocol to forward traffic to the endpoints.
		Please check http://blog.haproxy.com/haproxy/proxy-protocol
		Be sure that both endpoints of the connection support proxy protocol.
		`)

	// sysctl changes required by keepalived
	sysctlAdjustments = map[string]int{
		// allows processes to bind() to non-local IP addresses
		"net/ipv4/ip_nonlocal_bind": 1,
		// enable connection tracking for LVS connections
		"net/ipv4/vs/conntrack": 1,
	}

	vrid = flags.Int("vrid", 50,
		`The keepalived VRID (Virtual Router Identifier, between 0 and 255 as per 
      RFC-5798), which must be different for every Virtual Router (ie. every 
      keepalived sets) running on the same network.`)
)

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	flag.Set("logtostderr", "true")

	if *configMapName == "" {
		glog.Fatalf("Please specify --services-configmap")
	}

	if *vrid < 0 || *vrid > 255 {
		glog.Fatalf("Error using VRID %d, only values between 0 and 255 are allowed.", vrid)
	}

	if *useUnicast {
		glog.Info("keepalived will use unicast to sync the nodes")
	}

	kubeClient, err := createApiserverClient(*apiserverHost, *kubeConfigFile)
	if err != nil {
		handleFatalInitError(err)
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

	if *proxyMode {
		copyHaproxyCfg()
	}

	glog.Info("starting LVS configuration")
	ipvsc := newIPVSController(kubeClient, *watchNamespace, *useUnicast, *configMapName, *vrid, *proxyMode)

	go ipvsc.epController.Run(wait.NeverStop)
	go ipvsc.svcController.Run(wait.NeverStop)

	go ipvsc.syncQueue.run(time.Second, ipvsc.stopCh)

	go handleSigterm(ipvsc)

	glog.Info("starting keepalived to announce VIPs")
	ipvsc.keepalived.Start()
}

func handleSigterm(ipvsc *ipvsControllerController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	if err := ipvsc.Stop(); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}

const (
	// High enough QPS to fit all expected use cases. QPS=0 is not set here, because
	// client code is overriding it.
	defaultQPS = 1e6
	// High enough Burst to fit all expected use cases. Burst=0 is not set here, because
	// client code is overriding it.
	defaultBurst = 1e6
)

// buildConfigFromFlags builds REST config based on master URL and kubeconfig path.
// If both of them are empty then in cluster config is used.
func buildConfigFromFlags(masterURL, kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" && masterURL == "" {
		kubeconfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}

		return kubeconfig, nil
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmd_api.Cluster{
				Server: masterURL,
			},
		}).ClientConfig()
}

// createApiserverClient creates new Kubernetes Apiserver client. When kubeconfig or apiserverHost param is empty
// the function assumes that it is running inside a Kubernetes cluster and attempts to
// discover the Apiserver. Otherwise, it connects to the Apiserver specified.
//
// apiserverHost param is in the format of protocol://address:port/pathPrefix, e.g.http://localhost:8001.
// kubeConfig location of kubeconfig file
func createApiserverClient(apiserverHost string, kubeConfig string) (*kubernetes.Clientset, error) {
	cfg, err := buildConfigFromFlags(apiserverHost, kubeConfig)
	if err != nil {
		return nil, err
	}

	cfg.QPS = defaultQPS
	cfg.Burst = defaultBurst
	cfg.ContentType = "application/vnd.kubernetes.protobuf"

	glog.Infof("Creating API server client for %s", cfg.Host)

	client, err := kubernetes.NewForConfig(cfg)

	if err != nil {
		return nil, err
	}
	return client, nil
}

/**
 * Handles fatal init error that prevents server from doing any work. Prints verbose error
 * message and quits the server.
 */
func handleFatalInitError(err error) {
	glog.Fatalf("Error while initializing connection to Kubernetes apiserver. "+
		"This most likely means that the cluster is misconfigured (e.g., it has "+
		"invalid apiserver certificates or service accounts configuration). Reason: %s\n", err)
}
