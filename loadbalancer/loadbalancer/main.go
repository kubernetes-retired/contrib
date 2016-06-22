package main

import (
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/contrib/loadbalancer/loadbalancer/backend"
	_ "k8s.io/contrib/loadbalancer/loadbalancer/backend/backends"
	"k8s.io/contrib/loadbalancer/loadbalancer/controllers"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var (
	flags     = pflag.NewFlagSet("", pflag.ExitOnError)
	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	watchNamespace = flag.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Configmap/Services/Endpoints. By default the controller
		watches acrosss all namespaces`)
	backendName = flags.String("backend", "loadbalancer-daemon",
		`Backend to use. Default is loadbalancer-daemon.`)

	// Values to verify the configmap object is a loadbalancer config
	configLabelKey   = "app"
	configLabelValue = "loadbalancer"
)

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	clientConfig := util.DefaultClientConfig(flags)

	var kubeClient *unversioned.Client

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

	backendController, err := backend.CreateBackendController(kubeClient, *watchNamespace, map[string]string{
		"BACKEND": *backendName,
	}, configLabelKey, configLabelValue)
	if err != nil {
		glog.Fatalf("Could not create a backend controller for %v", *backendName)
	}
	loadBalancerController, _ := controllers.NewLoadBalancerController(kubeClient, 30*time.Second, *watchNamespace, backendController, configLabelKey, configLabelValue)
	loadBalancerController.Run()
}
