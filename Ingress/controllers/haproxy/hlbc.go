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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/util/proc"
	flag "github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	reloadQPS        = 10.0
	resyncPeriod     = 10 * time.Second
	lbApiPort        = 8081
	lbAlgorithmKey   = "serviceloadbalancer/lb.algorithm"
	lbHostKey        = "serviceloadbalancer/lb.host"
	defaultErrorPage = "file:///etc/haproxy/errors/404.http"
)

var (
	flags = flag.NewFlagSet("hlbc", flag.ExitOnError)

	// keyFunc for endpoints and services.
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc

	// Error used to indicate that a sync is deferred because the controller isn't ready yet
	errDeferredSync = fmt.Errorf("deferring sync till endpoints controller has synced")

	// See https://cbonte.github.io/haproxy-dconv/configuration-1.5.html#4.2-balance
	// In brief:
	//  * roundrobin: backend with the highest weight (how is this set?) receives new connection
	//  * leastconn: backend with least connections receives new connection
	//  * first: first server sorted by server id, with an available slot receives connection
	//  * source: connection given to backend based on hash of source ip
	supportedAlgorithms = []string{"roundrobin", "leastconn", "first", "source"}

	config = flags.String("cfg", "loadbalancer.json", `path to load balancer json config.
		Note that this is *not* the path to the configuration file for the load balancer
		itself, but rather, the path to the json configuration of how you would like the
		load balancer to behave in the kubernetes cluster.`)

	dry = flags.Bool("dry", false, `if set, a single dry run of configuration
		parsing is executed. Results written to stdout.`)

	cluster = flags.Bool("running-in-cluster", true, `Optional, if this controller is
		running in a Kubernetes cluster, use the pod secrets for creating the client.`)

	// If you have pure tcp services or https services that need L3 routing, you
	// must specify them by name. Note that you are responsible for:
	// 1. Making sure there is no collision between the service ports of these services.
	//	- You can have multiple <mysql svc name>:3306 specifications in this map, and as
	//	  long as the service ports of your mysql service don't clash, you'll get
	//	  loadbalancing for each one.
	// 2. Exposing the service ports as node ports on a pod.
	// 3. Adding firewall rules so these ports can ingress traffic.
	//
	// Any service not specified in this map is treated as an http:80 service,
	// unless TargetService dictates otherwise.

	tcpServices = flags.String("tcp-services", "", `Comma separated list of tcp/https
		serviceName:servicePort pairings. This assumes you've opened up the right
		hostPorts for each service that serves ingress traffic.`)

	targetService = flags.String(
		"target-service", "", `Restrict loadbalancing to a single target service.`)

	// ForwardServices == true:
	// The lb just forwards packets to the vip of the service and we use
	// kube-proxy's inbuilt load balancing. You get rules:
	// backend svc_p1: svc_ip:p1
	// backend svc_p2: svc_ip:p2
	//
	// ForwardServices == false:
	// The lb is configured to match up services to endpoints. So for example,
	// you have (svc:p1, p2 -> tp1, tp2) we essentially get all endpoints with
	// the same targetport and create a new svc backend for them, i.e:
	// backend svc_p1: pod1:tp1, pod2:tp1
	// backend svc_p2: pod1:tp2, pod2:tp2

	forwardServices = flags.Bool("forward-services", false, `Forward to service vip
		instead of endpoints. This will use kube-proxy's inbuilt load balancing.`)

	httpPort  = flags.Int("http-port", 80, `Port to expose http services.`)
	statsPort = flags.Int("stats-port", 1936, `Port for loadbalancer stats,
		Used in the loadbalancer liveness probe.`)

	startSyslog = flags.Bool("syslog", false, `if set, it will start a syslog server
		that will forward haproxy logs to stdout.`)

	errorPage = flags.String("error-page", "", `if set, it will try to load the content
		as a web page and use the content as error page. Is required that the URL returns
		200 as a status code`)

	lbDefAlgorithm = flags.String("balance-algorithm", "roundrobin", `if set, it allows a custom
		default balance algorithm.`)

	useIngress = flags.Bool("use-ingress", false, `if set, the Ingress resource is consulted
    	to get route information for a Service (eg: host, path).`)

	watchAll = flags.Bool("watch-all-namespaces", false, `if set, the controller watches all
		namespaces. Otherwise it only watches the namespace specifiec via -namespace, and
		default if none is specified.`)
)

// service encapsulates a single backend entry in the load balancer config.
// The Ep field can contain the ips of the pods that make up a service, or the
// clusterIP of the service itself (in which case the list has a single entry,
// and kubernetes handles loadbalancing across the service endpoints).
type service struct {
	Name string
	Ep   []string

	// FrontendPort is the port that the loadbalancer listens on for traffic
	// for this service. For http, it's always :80, for each tcp service it
	// is the service port of any service matching a name in the tcpServices set.
	FrontendPort int

	// Host if not empty it will add a new haproxy acl to route traffic using the
	// host header inside the http request. It only applies to http traffic.
	Host string

	// Algorithm
	Algorithm string
}

// loadBalancerConfig represents loadbalancer specific configuration. Eventually
// kubernetes will have an api for l7 loadbalancing.
type loadBalancerConfig struct {
	Name           string `json:"name" description:"Name of the load balancer, eg: haproxy."`
	ReloadCmd      string `json:"reloadCmd" description:"command used to reload the load balancer."`
	Config         string `json:"config" description:"path to loadbalancers configuration file."`
	Template       string `json:"template" description:"template for the load balancer config."`
	Algorithm      string `json:"algorithm" description:"loadbalancing algorithm."`
	startSyslog    bool   `description:"indicates if the load balancer uses syslog."`
	lbDefAlgorithm string `description:"custom default load balancer algorithm".`
}

type staticPageHandler struct {
	pagePath     string
	pageContents []byte
	c            *http.Client
}

type serviceAnnotations map[string]string

func (s serviceAnnotations) getAlgorithm() (string, bool) {
	val, ok := s[lbAlgorithmKey]
	return val, ok
}

func (s serviceAnnotations) getHost() (string, bool) {
	val, ok := s[lbHostKey]
	return val, ok
}

// Get serves the error page
func (s *staticPageHandler) Getfunc(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	w.Write(s.pageContents)
}

// newStaticPageHandler returns a staticPageHandles with the contents of pagePath loaded and ready to serve
func newStaticPageHandler(errorPage string, defaultErrorPage string) *staticPageHandler {
	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	c := &http.Client{Transport: t}
	s := &staticPageHandler{c: c}
	if err := s.loadUrl(errorPage); err != nil {
		s.loadUrl(defaultErrorPage)
	}

	return s
}

func (s *staticPageHandler) loadUrl(url string) error {
	res, err := s.c.Get(url)
	if err != nil {
		glog.Errorf("%v", err)
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		glog.Errorf("%v", err)
		return err
	}
	glog.V(2).Infof("Error page:\n%v", string(body))
	s.pagePath = url
	s.pageContents = body

	return nil
}

// write writes the configuration file, will write to stdout if dryRun == true
func (cfg *loadBalancerConfig) write(services map[string][]service, dryRun bool) (err error) {
	var w io.Writer
	if dryRun {
		w = os.Stdout
	} else {
		w, err = os.Create(cfg.Config)
		if err != nil {
			return
		}
	}
	var t *template.Template
	t, err = template.ParseFiles(cfg.Template)
	if err != nil {
		return
	}

	conf := make(map[string]interface{})
	conf["startSyslog"] = strconv.FormatBool(cfg.startSyslog)
	conf["services"] = services

	// default load balancer algorithm is roundrobin
	conf["defLbAlgorithm"] = lbDefAlgorithm
	if cfg.lbDefAlgorithm != "" {
		conf["defLbAlgorithm"] = cfg.lbDefAlgorithm
	}

	return t.Execute(w, conf)
}

// reload reloads the loadbalancer using the reload cmd specified in the json manifest.
func (cfg *loadBalancerConfig) reload() error {
	output, err := exec.Command("sh", "-c", cfg.ReloadCmd).CombinedOutput()
	msg := fmt.Sprintf("%v -- %v", cfg.Name, string(output))
	if err != nil {
		return fmt.Errorf("error restarting %v: %v", msg, err)
	}
	glog.Infof(msg)
	return nil
}

// loadBalancerController watches the kubernetes api and adds/removes services
// from the loadbalancer, via loadBalancerConfig.
type loadBalancerController struct {
	cfg               *loadBalancerConfig
	queue             *workqueue.Type
	client            *client.Client
	epController      *framework.Controller
	svcController     *framework.Controller
	ingController     *framework.Controller
	ingStore          cache.Store
	svcLister         cache.StoreToServiceLister
	epLister          cache.StoreToEndpointsLister
	reloadRateLimiter util.RateLimiter
	template          string
	targetService     string
	forwardServices   bool
	tcpServices       map[string]int
	httpPort          int
	useIngress        bool
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (lbc *loadBalancerController) getEndpoints(
	s *api.Service, servicePort *api.ServicePort) (endpoints []string) {
	ep, err := lbc.epLister.GetServiceEndpoints(s)
	if err != nil {
		return
	}

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of servicing traffic for it.
	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {
			var targetPort int
			switch servicePort.TargetPort.Kind {
			case util.IntstrInt:
				if epPort.Port == servicePort.TargetPort.IntVal {
					targetPort = epPort.Port
				}
			case util.IntstrString:
				if epPort.Name == servicePort.TargetPort.StrVal {
					targetPort = epPort.Port
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range ss.Addresses {
				endpoints = append(endpoints, fmt.Sprintf("%v:%v", epAddress.IP, targetPort))
			}
		}
	}
	return
}

// encapsulates all the hacky convenience type name modifications for lb rules.
// - :80 services don't need a :80 postfix
// - default ns should be accessible without /ns/name (when we have /ns support)
func getServiceNameForLBRule(serviceName string, servicePort api.ServicePort) string {
	if servicePort.Port != 0 {
		if servicePort.Port == 80 {
			return serviceName
		}
		return fmt.Sprintf("%v:%v", serviceName, servicePort.Port)
	}
	return fmt.Sprintf("%v:%v", serviceName, servicePort.Name)
}

// getServices returns a list of services and their endpoints.
func (lbc *loadBalancerController) getServices() (httpSvc []service, tcpSvc []service) {
	ep := []string{}
	services, _ := lbc.svcLister.List()
	for _, s := range services.Items {
		if s.Spec.Type == api.ServiceTypeLoadBalancer {
			glog.Infof("Ignoring service %v, it already has a loadbalancer", s.Name)
			continue
		}
		for _, servicePort := range s.Spec.Ports {
			// TODO: headless services?
			sName := s.Name
			if servicePort.Protocol == api.ProtocolUDP ||
				(lbc.targetService != "" && lbc.targetService != sName) {
				glog.Infof("Ignoring %v: %+v", sName, servicePort)
				continue
			}

			if lbc.forwardServices {
				ep = []string{
					fmt.Sprintf("%v:%v", s.Spec.ClusterIP, servicePort.Port)}
			} else {
				ep = lbc.getEndpoints(&s, &servicePort)
			}
			if len(ep) == 0 {
				glog.Infof("No endpoints found for service %v, port %+v",
					sName, servicePort)
				continue
			}
			newSvc := service{
				Name: getServiceNameForLBRule(s.Name, servicePort),
				Ep:   ep,
			}

			if val, ok := serviceAnnotations(s.ObjectMeta.Annotations).getHost(); ok {
				newSvc.Host = val
			}

			if val, ok := serviceAnnotations(s.ObjectMeta.Annotations).getAlgorithm(); ok {
				for _, current := range supportedAlgorithms {
					if val == current {
						newSvc.Algorithm = val
						break
					}
				}
			}

			if port, ok := lbc.tcpServices[sName]; ok && port == servicePort.Port {
				newSvc.FrontendPort = servicePort.Port
				tcpSvc = append(tcpSvc, newSvc)
			} else {
				newSvc.FrontendPort = lbc.httpPort
				httpSvc = append(httpSvc, newSvc)
			}
			glog.Infof("Found service: %+v", newSvc)
		}
	}
	if lbc.useIngress {
		// Overwrite the path/host rules if http services based on Ingress.
		httpSvc = lbc.fillIngressRules(httpSvc)
	}
	return
}

// fillIngressRules writes rules from the Ingress to the Service.
// If no Ingress exists for a Service:port, it's discarded.
func (lbc *loadBalancerController) fillIngressRules(svcs []service) []service {
	svcLookup := map[string]service{}
	for _, svc := range svcs {
		svcLookup[svc.Name] = svc
	}
	out := []service{}
	for _, m := range lbc.ingStore.List() {
		ing := *m.(*extensions.Ingress)
		for _, rules := range ing.Spec.Rules {
			if rules.IngressRuleValue.HTTP == nil {
				continue
			}
			for _, p := range rules.IngressRuleValue.HTTP.Paths {
				servicePort := api.ServicePort{}
				sp := p.Backend.ServicePort
				switch sp.Kind {
				case util.IntstrInt:
					servicePort.Port = sp.IntVal
				case util.IntstrString:
					servicePort.Name = sp.StrVal
				}
				name := getServiceNameForLBRule(
					p.Backend.ServiceName, servicePort)
				s, ok := svcLookup[name]
				if !ok {
					glog.Infof("Service %v has no Ingress, ignoring", name)
					continue
				}
				if path, err := sanitizePath(p.Path); err != nil {
					// TODO: Add an event here.
					glog.Infof("%v: %v is invalid, ignoring", ing.Name, path)
					continue
				} else {
					s.Name = path
				}
				s.Host = rules.Host
				out = append(out, s)
			}
		}
	}
	return out
}

// sanitizePath removes leading and trailing slashes, and checks if the given
// path is a PCRE regex.
func sanitizePath(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	_, err := regexp.Compile(path)
	return path, err
}

// sync all services with the loadbalancer.
func (lbc *loadBalancerController) sync(dryRun bool) error {
	if !lbc.epController.HasSynced() || !lbc.svcController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
		return errDeferredSync
	}
	httpSvc, tcpSvc := lbc.getServices()
	if len(httpSvc) == 0 && len(tcpSvc) == 0 {
		return nil
	}
	if err := lbc.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, dryRun); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	lbc.reloadRateLimiter.Accept()
	return lbc.cfg.reload()
}

// worker handles the work queue.
func (lbc *loadBalancerController) worker() {
	for {
		key, _ := lbc.queue.Get()
		glog.Infof("Sync triggered by service %v", key)
		if err := lbc.sync(false); err != nil {
			glog.Infof("Requeuing %v because of error: %v", key, err)
			lbc.queue.Add(key)
		}
		lbc.queue.Done(key)
	}
}

// newLoadBalancerController creates a new controller from the given config.
func newLoadBalancerController(cfg *loadBalancerConfig, kubeClient *client.Client, namespace string) *loadBalancerController {

	lbc := loadBalancerController{
		cfg:    cfg,
		client: kubeClient,
		queue:  workqueue.New(),
		reloadRateLimiter: util.NewTokenBucketRateLimiter(
			reloadQPS, int(reloadQPS)),
		targetService:   *targetService,
		forwardServices: *forwardServices,
		httpPort:        *httpPort,
		tcpServices:     map[string]int{},
	}

	for _, service := range strings.Split(*tcpServices, ",") {
		portSplit := strings.Split(service, ":")
		if len(portSplit) != 2 {
			glog.Errorf("Ignoring misconfigured TCP service %v", service)
			continue
		}
		if port, err := strconv.Atoi(portSplit[1]); err != nil {
			glog.Errorf("Ignoring misconfigured TCP service %v: %v", service, err)
			continue
		} else {
			lbc.tcpServices[portSplit[0]] = port
		}
	}
	enqueue := func(obj interface{}) {
		key, err := keyFunc(obj)
		if err != nil {
			glog.Infof("Couldn't get key for object %+v: %v", obj, err)
			return
		}
		lbc.queue.Add(key)
	}
	eventHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc:    enqueue,
		DeleteFunc: enqueue,
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				enqueue(cur)
			}
		},
	}

	lbc.svcLister.Store, lbc.svcController = framework.NewInformer(
		cache.NewListWatchFromClient(
			lbc.client, "services", namespace, fields.Everything()),
		&api.Service{}, resyncPeriod, eventHandlers)

	lbc.epLister.Store, lbc.epController = framework.NewInformer(
		cache.NewListWatchFromClient(
			lbc.client, "endpoints", namespace, fields.Everything()),
		&api.Endpoints{}, resyncPeriod, eventHandlers)

	lbc.ingStore, lbc.ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(lbc.client),
			WatchFunc: ingressWatchFunc(lbc.client),
		},
		&extensions.Ingress{}, resyncPeriod, framework.ResourceEventHandlerFuncs{})

	return &lbc
}

func ingressListFunc(c *client.Client) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		return c.Extensions().Ingress(api.NamespaceAll).List(labels.Everything(), fields.Everything())
	}
}

func ingressWatchFunc(c *client.Client) func(rv string) (watch.Interface, error) {
	return func(rv string) (watch.Interface, error) {
		return c.Extensions().Ingress(api.NamespaceAll).Watch(
			labels.Everything(), fields.Everything(), rv)
	}
}

// parseCfg parses the given configuration file.
// cmd line params take precedence over config directives.
func parseCfg(configPath string, defLbAlgorithm string) *loadBalancerConfig {
	jsonBlob, err := ioutil.ReadFile(configPath)
	if err != nil {
		glog.Fatalf("Could not parse lb config: %v", err)
	}
	var cfg loadBalancerConfig
	err = json.Unmarshal(jsonBlob, &cfg)
	if err != nil {
		glog.Fatalf("Unable to unmarshal json blob: %v", string(jsonBlob))
	}

	cfg.lbDefAlgorithm = defLbAlgorithm
	glog.Infof("Creating new loadbalancer: %+v", cfg)
	return &cfg
}

// registerHandlers  services liveness probes.
func registerHandlers(s *staticPageHandler) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Delegate a check to the haproxy stats service.
		response, err := http.Get(fmt.Sprintf("http://localhost:%v", *statsPort))
		if err != nil {
			glog.Infof("Error %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				contents, err := ioutil.ReadAll(response.Body)
				if err != nil {
					glog.Infof("Error reading resonse on receiving status %v: %v",
						response.StatusCode, err)
				}
				glog.Infof("%v\n", string(contents))
				w.WriteHeader(response.StatusCode)
			} else {
				w.WriteHeader(200)
				w.Write([]byte("ok"))
			}
		}
	})

	// handler for not matched traffic
	http.HandleFunc("/", s.Getfunc)

	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", lbApiPort), nil))
}

func dryRun(lbc *loadBalancerController) {
	var err error
	for err = lbc.sync(true); err == errDeferredSync; err = lbc.sync(true) {
	}
	if err != nil {
		glog.Infof("ERROR: %+v", err)
	}
}

func main() {
	clientConfig := kubectl_util.DefaultClientConfig(flags)
	flags.Parse(os.Args)
	cfg := parseCfg(*config, *lbDefAlgorithm)
	if len(*tcpServices) == 0 {
		glog.Infof("All tcp/https services will be ignored.")
	}

	var kubeClient *client.Client
	var err error

	defErrorPage := newStaticPageHandler(*errorPage, defaultErrorPage)
	if defErrorPage == nil {
		glog.Fatalf("Failed to load the default error page")
	}

	go registerHandlers(defErrorPage)

	proc.StartReaper()

	if *startSyslog {
		cfg.startSyslog = true
		_, err = newSyslogServer("/var/run/haproxy.log.socket")
		if err != nil {
			glog.Fatalf("Failed to start syslog server: %v", err)
		}
	}

	if *cluster {
		if kubeClient, err = client.NewInCluster(); err != nil {
			glog.Fatalf("Failed to create client: %v", err)
		}
	} else {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = client.New(config)
	}
	var namespace string
	if *watchAll {
		namespace = api.NamespaceAll
		// One can't watch servcies in all namespaces because of collisions.
		// i.e an Ingress specifically configures a loadbalancer behind a
		// given IP with some path rules. This has nothing to do with
		// namespaces. However, if 2 services with the same name show up in
		// different namespaces, which one gets the service name path? This
		// is a case of explicit (Ingress) vs implicit (service name), where
		// we simply don't handle the implicit case.
		if !*useIngress {
			glog.Fatalf(
				"Invalid configuration: --use-ingress=false, --watch-all-namespaces=true.")
		}
	} else {
		var specified bool
		var err error
		namespace, specified, err = clientConfig.Namespace()
		if err != nil {
			glog.Fatalf("unexpected error: %v", err)
		}
		if !specified {
			namespace = "default"
		}
	}
	// TODO: Handle multiple namespaces
	lbc := newLoadBalancerController(cfg, kubeClient, namespace)
	go lbc.epController.Run(util.NeverStop)
	go lbc.svcController.Run(util.NeverStop)
	if *useIngress {
		lbc.useIngress = true
		go lbc.ingController.Run(util.NeverStop)
	}
	if *dry {
		dryRun(lbc)
	} else {
		lbc.cfg.reload()
		util.Until(lbc.worker, time.Second, util.NeverStop)
	}
}
