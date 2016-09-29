package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/contrib/ingress/controllers/haproxy/cmd/model"
	"k8s.io/contrib/ingress/controllers/haproxy/cmd/server"
	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer/haproxy"
	"k8s.io/contrib/ingress/controllers/haproxy/pkg/controller"
	"github.com/golang/glog"
	log "github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	defaultAPIPort             = 8207
	defaultListen              = "0.0.0.0"
	defaultOutCluster          = false
	defaultWatchNamespace      = api.NamespaceAll
	defaultBalancerNamespace   = api.NamespaceDefault
	defaultResync              = 30 * time.Second
	defaultLoadBalancer        = "haproxy"
	defaultConfigMap           = "loadbalancer-conf"
	defaultIngressFilterNeeded = false
)

var (
	flags      = pflag.NewFlagSet("", pflag.ExitOnError)
	port       = flags.Int("api-port", defaultAPIPort, "Port to expose the Ingress Controller API")
	listen     = flags.String("listen", defaultListen, "IP to expose Haddok API")
	outCluster = flags.Bool("out-of-cluster", defaultOutCluster, "If true the cluster balancer is not within the cluster")

	balancerNamespace   = flags.String("balancer-pod-namespace", defaultBalancerNamespace, "namespace where the balancer is running")
	balancerPodName     = flags.String("balancer-pod-name", "", "balancer pod name")
	watchNamespace      = flags.String("watch-namespace", defaultWatchNamespace, "namespace to watch")
	resyncPeriod        = flags.Duration("sync-period", defaultResync, "Relist Ingress frequency")
	balancerIP          = flags.String("balancer-ip", "", "Public IP Address where the balancer is located")
	loadBalancer        = flags.String("load-balancer", defaultLoadBalancer, "Load Balancer to configure. For now only 'haproxy' is supported")
	balancerScript      = flags.String("balancer-script", "", "Load balancer shell script that accepts commands as parameters")
	certsDir            = flags.String("certs-dir", "", "Directory to store TLS certificates")
	configMap           = flags.String("config-map", defaultConfigMap, "Configuration for global balancer items")
	configFile          = flags.String("config-file", "", "Load Balancer configuration file location")
	ingressFilter       = flags.StringSlice("ingress-class-filter", nil, "Group of filter values for kubernetes.io/ingress.class ingress annotations accepted by this balancer.")
	ingressFilterNeeded = flags.Bool("ingress-class-needed", defaultIngressFilterNeeded, "If set to true, will only process annotations indicated by 'ingress-class-filter'")
)

func main() {

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	log.Infof("starting HAProxy ingress controller: %+v", model.GetAppInfo())

	if *ingressFilterNeeded && len(*ingressFilter) == 0 {
		log.Fatal("using 'ingress-annotation-allow-needed' requires informing 'ingress-annotation-filter'")
	}

	c, e := getKubeclient()
	if e != nil {
		log.Fatalf("error connecting to kubernetes cluster: %+v", e)
	}

	*balancerIP, e = getBalancerIP(c)
	if e != nil {
		log.Fatalf("error getting balancer IP: %+v", e)
	}

	lbManager, e := getLBManager(*configFile, *balancerScript, *certsDir)
	if e != nil {
		log.Fatalf("error choosing load balancer manager: %v", e)
	}

	if e = lbManager.StartBalancer(); e != nil {
		log.Fatalf("couldn't start load balancer: %v", e)
	}

	lbc := controller.NewLoadBalancerController(c, *watchNamespace, *resyncPeriod, *balancerIP, lbManager, *configMap, *ingressFilterNeeded, *ingressFilter)

	go handleSigterm(lbc)
	go server.StartServer(*listen, *port, &lbManager)

	lbc.Run()
}

// getKubeclient returns the kubeclient using the flag that indicates if it's in cluster or out of cluster
func getKubeclient() (*client.Client, error) {
	defaultCfg := util.DefaultClientConfig(flags)

	if !*outCluster {
		log.Info("creating in-cluster client")
		c, e := client.NewInCluster()
		if e != nil {
			return nil, e
		}
		return c, nil
	}

	cfg, e := defaultCfg.ClientConfig()
	if e != nil {
		return nil, e
	}

	log.Info("creating out-of-cluster client")
	c, e := client.New(cfg)
	if e != nil {
		return nil, e
	}

	return c, nil
}

func getLBManager(configFile string, balancerScript string, certsDir string) (balancer.Manager, error) {
	log.Infof("getLBManager: configFile=%s balancerScript=%s", configFile, balancerScript)

	if *loadBalancer == "haproxy" {
		return haproxy.NewManager(configFile, balancerScript, certsDir)
	}
	return nil, fmt.Errorf("not a valid load balancer manager '%s'", *loadBalancer)
}

func getBalancerIP(c *client.Client) (string, error) {
	if *balancerIP != "" {
		return *balancerIP, nil
	}

	if *outCluster {
		return "", fmt.Errorf("out of cluster balancers must set the balancer IP")
	}

	return getPodIP(c)
}

func getPodIP(c *client.Client) (string, error) {
	ns := *balancerNamespace
	if ns == "" {
		ns = "default"
	}

	log.V(2).Infof("getting pod info for %s", *balancerPodName)
	pod, e := c.Pods(ns).Get(*balancerPodName)
	if e != nil {
		return "", fmt.Errorf("could not get %s/%s info: %+v", ns, *balancerPodName, e)
	}

	var node *api.Node
	log.Infof("getting node info for %s", pod.Spec.NodeName)
	node, e = c.Nodes().Get(pod.Spec.NodeName)
	if e != nil {
		return "", fmt.Errorf("could not get node %s info: %+v", pod.Spec.NodeName, e)
	}

	var externalIP string

	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeExternalIP && address.Address != "" {
			externalIP = address.Address
			break
		}
		if externalIP == "" && address.Type == api.NodeLegacyHostIP {
			externalIP = address.Address
		}
	}

	if externalIP == "" {
		return "", fmt.Errorf("could not get node %s external IP", pod.Spec.NodeName)
	}

	return externalIP, nil
}

func handleSigterm(lbc *controller.LoadBalancerController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	log.Infof("Received SIGTERM, shutting down")

	if err := lbc.Stop(); err != nil {
		glog.Fatalf("Error during shutdown %v", err)
	}
	os.Exit(0)
}
