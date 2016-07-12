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
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
)

var (
	flags     = pflag.NewFlagSet("", pflag.ExitOnError)
	inCluster = flags.Bool("running-in-cluster", true,
		`Optional, if this controller is running in a kubernetes cluster, use the
		 pod secrets for creating a Kubernetes client.`)

	watchNamespace = flag.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Configmap/Services/Endpoints. By default the controller
		watches acrosss all namespaces`)
	backendName = flags.String("backend", "openstack-lbaasv2",
		`Backend to use. Default is openstack-lbaasv2.`)
)

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	kubeClient := utils.GetK8Client(utils.ClientConfig{
		InCluster: *inCluster,
		Namespace: *watchNamespace,
		Flags:     flags,
	})

	backendController, err := backend.CreateBackendController(map[string]string{
		"BACKEND": *backendName,
	})
	if err != nil {
		glog.Fatalf("Could not create a backend controller for %v", *backendName)
	}
	loadBalancerController, _ := controllers.NewLoadBalancerController(kubeClient, 30*time.Second, backendController)
	loadBalancerController.Run()
}
