package controller

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
	log "github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
)

const (

	// time to wait if the controllers have not been synched at initialization
	storeSyncPollPeriod = 5 * time.Second
)

// LoadBalancerController is a kubernetes load balancer controller
type LoadBalancerController struct {
	client                    *client.Client
	lbManager                 balancer.Manager
	namespace                 string
	balancerIP                string
	configMapName             string
	resyncPeriod              time.Duration
	ingressFilterChecksNeeded bool
	ingressFilter             []string

	ingController    *framework.Controller
	svcController    *framework.Controller
	endpController   *framework.Controller
	secretController *framework.Controller

	ingressStore cache.Store
	svcLister    cache.StoreToServiceLister
	endpLister   cache.StoreToEndpointsLister
	secretStore  cache.Store

	ingQueue  *taskQueue
	syncQueue *taskQueue

	recorder record.EventRecorder

	stopCh chan struct{}

	stopLock sync.Mutex
	shutdown bool
}

// NewLoadBalancerController creates a new LoadBalancerController
// TODO balancerIP should be a string array
func NewLoadBalancerController(client *client.Client, namespace string, resyncPeriod time.Duration, balancerIP string, lbManager balancer.Manager, configMapLB string, ingressFilterNeeded bool, ingressFilter []string) *LoadBalancerController {
	log.Infof("creating new load balancer. namespace: '%s' resyncPeriod: '%s', balancerIP: '%s', configMapLB: '%s'", namespace, resyncPeriod, balancerIP, configMapLB)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(client.Events(namespace))

	lbc := LoadBalancerController{
		client:        client,
		lbManager:     lbManager,
		stopCh:        make(chan struct{}),
		namespace:     namespace,
		configMapName: configMapLB,
		resyncPeriod:  resyncPeriod,
		balancerIP:    balancerIP,
		recorder:      eventBroadcaster.NewRecorder(api.EventSource{Component: "loadbalancer-ingress-controller"}),
	}

	// if ingress filter is not needed and no filter has been specified, it's not necessary to filter ingresses
	if !ingressFilterNeeded && len(ingressFilter) == 0 {
		lbc.ingressFilterChecksNeeded = false
	} else {
		lbc.ingressFilterChecksNeeded = true
		// if we received a list of filters, but also allow no annotations, add empty string to the fliter array
		if !ingressFilterNeeded {
			lbc.ingressFilter = append(ingressFilter, "")
		}
	}

	lbc.syncQueue = newTaskQueue(lbc.syncConfig)
	lbc.ingQueue = newTaskQueue(lbc.updateIngress)

	// creates informers
	lbc.setIngressInformer()
	lbc.setServiceInformer()
	lbc.setEndPointInformer()
	lbc.setSecretInformer()

	return &lbc
}

// Run starts the load balancer control loop
func (lbc *LoadBalancerController) Run() {
	log.Info("running the load balancer")

	// start controllers
	go lbc.ingController.Run(lbc.stopCh)
	go lbc.svcController.Run(lbc.stopCh)
	go lbc.endpController.Run(lbc.stopCh)
	go lbc.secretController.Run(lbc.stopCh)

	// start queue processors
	go lbc.syncQueue.run(time.Second, lbc.stopCh)
	go lbc.ingQueue.run(time.Second, lbc.stopCh)

	<-lbc.stopCh
	log.Info("load balancer stopped")
}

// Stop stops the loadbalancer controller
func (lbc *LoadBalancerController) Stop() error {
	log.Info("stopping the load balancer")
	lbc.stopLock.Lock()
	defer lbc.stopLock.Unlock()

	// Only try draining the workqueue if we haven't already.
	if !lbc.shutdown {
		close(lbc.stopCh)
		log.Infof("shutting down balancer controller queues")
		lbc.ingQueue.shutdown()
		lbc.syncQueue.shutdown()
		lbc.shutdown = true
		return nil
	}

	return fmt.Errorf("shutdown is already in progress")
}

// syncConfig creates a new config bundle and sends it to the lbManager config writer and restarter
func (lbc *LoadBalancerController) syncConfig(key string) error {
	log.Infof("synchronizing config task executing for key: %s", key)

	if !lbc.allControllersSynched() {
		time.Sleep(storeSyncPollPeriod)
		return fmt.Errorf("deferring sync until all controllers have synced")
	}

	cfg, e := lbc.getConfigFromIngress()
	if e != nil {
		return e
	}

	log.V(4).Infof("read config from ingress: %+v", cfg)
	e = lbc.lbManager.WriteConfigAndRestart(cfg, false)

	return e
}

// getConfigFromIngress builds a balancer.Config object from the ingresses
func (lbc *LoadBalancerController) getConfigFromIngress() (*balancer.Config, error) {
	log.Info("building config from kubernetes ingresses")
	ings := lbc.ingressStore.List()

	cfg := &balancer.Config{}
	cfg.Exposed = make(map[string]balancer.Exposed)
	cfg.Upstreams = make(map[string]balancer.Upstream)

	for _, i := range ings {
		ing := *i.(*extensions.Ingress)

		if lbc.ingressFilterChecksNeeded && !ingAnnotations(ing.ObjectMeta.Annotations).filterClass(lbc.ingressFilter) {
			log.Infof("ignoring config process for ingress '%v' based on class annotation filtering", ing.Name)
			continue
		}

		var certs []balancer.Certificate
		for _, tls := range ing.Spec.TLS {
			cert, e := lbc.getCertificateFromSecret(tls.SecretName)
			if e != nil {
				log.Errorf("error retrieving certificate for secret '%s' for ingress '%s/%s': %+v", tls.SecretName, ing.GetNamespace(), ing.GetName(), e)
			}
			if cert != nil {
				certs = append(certs, *cert)
			}
		}

		secure := len(certs) != 0

		// process default backend
		backend := ing.Spec.Backend
		if backend != nil {

			endp, e := lbc.getServicePortEndPoints(fmt.Sprintf("%v/%v", ing.GetNamespace(), backend.ServiceName),
				backend.ServicePort.String(), api.ProtocolTCP)
			if e != nil {
				log.Errorf("no enpoint found for ingress '%s/%s' service '%[1]s/%s': %+v", ing.GetNamespace(), ing.GetName(), backend.ServiceName, e)
				continue
			}

			upstream := balancer.Upstream{
				Name:      backend.ServiceName + backend.ServicePort.String(),
				Endpoints: endp,
			}

			exposed := balancer.Exposed{
				Upstream:  &upstream,
				IsDefault: true,
			}
			if secure {
				exposed.Certificates = certs
				exposed.BindPort = 443
			} else {
				exposed.BindPort = 80
			}

			if _, ok := cfg.Exposed[exposed.Name()]; ok {
				log.Errorf("ingress '%s/%s' service '%[1]s/%s' contains duplicated information: %s", ing.GetNamespace(), ing.GetName(), backend.ServiceName, exposed.Name())
			} else {
				cfg.Exposed[exposed.Name()] = exposed
				cfg.Upstreams[upstream.Name] = upstream
			}
		}

		// process ingress rules
		rules := ing.Spec.Rules
		for _, rule := range rules {

			for _, p := range rule.HTTP.Paths {

				endp, e := lbc.getServicePortEndPoints(fmt.Sprintf("%v/%v", ing.GetNamespace(), p.Backend.ServiceName),
					p.Backend.ServicePort.String(), api.ProtocolTCP)
				if e != nil {
					log.Errorf("no enpoint found for ingress (rule) '%s/%s' service '%[1]s/%s': %+v", ing.GetNamespace(), ing.GetName(), p.Backend.ServiceName, e)
					continue
				}

				upstream := balancer.Upstream{
					Name:      p.Backend.ServiceName + p.Backend.ServicePort.String(),
					Endpoints: endp,
				}

				exposed := balancer.Exposed{
					HostName:   rule.Host,
					PathBegins: p.Path,
					Upstream:   &upstream,
					IsDefault:  false,
				}

				if secure {
					exposed.Certificates = certs
					exposed.BindPort = 443
				} else {
					exposed.BindPort = 80
				}

				if _, ok := cfg.Exposed[exposed.Name()]; ok {
					log.Errorf("ingress '%s/%s' service '%[1]s/%s' contains duplicated information: %s", ing.GetNamespace(), ing.GetName(), backend.ServiceName, exposed.Name())
				} else {
					cfg.Exposed[exposed.Name()] = exposed
					cfg.Upstreams[upstream.Name] = upstream
				}
			}
		}
	}

	return cfg, nil
}

// getServicePortEndPoints returns endpoints associated to a service/port
func (lbc *LoadBalancerController) getServicePortEndPoints(svcKey string, port string, proto api.Protocol) ([]balancer.Endpoint, error) {
	log.Infof("getting endpoints for service: '%s' port='%s' protocol='%s'", svcKey, port, proto)

	s, exists, e := lbc.svcLister.GetByKey(svcKey)
	if e != nil {
		return nil, fmt.Errorf("error getting service '%s' from store: %v", svcKey, e)
	}
	if !exists {
		return nil, fmt.Errorf("service '%s' wasn't found in the store", svcKey)
	}

	// get service port name   addressed by ingress
	// can be empty if there is only one port
	svc := s.(*api.Service)
	var portName string
	var targetPort int
	found := false
	for _, p := range svc.Spec.Ports {
		// We used to have this check: if int(p.Port) == port && p.Protocol == proto {
		// but we want to match nginx Ingress as much as possible, so we change it to:
		if strconv.Itoa(int(p.Port)) == port || p.TargetPort.String() == port || p.Name == port {
			portName = p.Name
			targetPort = p.TargetPort.IntValue()
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("service '%s' is not exposing on port %s", svcKey, port)
	}

	eps, e := lbc.endpLister.GetServiceEndpoints(svc)
	if e != nil {
		return nil, fmt.Errorf("error getting endpoints for service '%s': %v", svcKey, e)
	}

	endpoints := []balancer.Endpoint{}
	for _, epss := range eps.Subsets {
		// search port name here
		found = false
		for _, epPort := range epss.Ports {
			// check name, port and protocol match
			if epPort.Name == portName &&
				epPort.Protocol == proto &&
				int(epPort.Port) == targetPort {
				found = true
				break
			}
		}

		if !found {
			continue
		}

		for _, epAddr := range epss.Addresses {
			endpoints = append(endpoints, balancer.Endpoint{IP: epAddr.IP, Port: targetPort})
		}
	}

	return endpoints, nil

}

// updateIngress updates Ingress object with balancer IP
func (lbc *LoadBalancerController) updateIngress(key string) error {
	log.Infof("updating ingress element with balancer IP: '%s'", key)

	if !lbc.allControllersSynched() {
		time.Sleep(storeSyncPollPeriod)
		return fmt.Errorf("deferring ingress update untill all controllers have synched")
	}

	obj, ingExists, err := lbc.ingressStore.GetByKey(key)
	if err != nil {
		return err
	}

	if !ingExists {
		log.Warningf("couldn't find ingress with the key: %v", key)
		return nil
	}

	ing := obj.(*extensions.Ingress)

	ingClient := lbc.client.Extensions().Ingress(ing.Namespace)
	currIng, err := ingClient.Get(ing.Name)
	if err != nil {
		return fmt.Errorf("unexpected error searching Ingress '%s/%s': %v", ing.Namespace, ing.Name, err)
	}

	if !lbc.containsBalancerIP(ing.Status.LoadBalancer.Ingress) {
		log.Infof("updating ingress with balancer IP: '%s' ingress: '%s/'", lbc.balancerIP, ing.Namespace, ing.Name)

		currIng.Status.LoadBalancer.Ingress = append(currIng.Status.LoadBalancer.Ingress, api.LoadBalancerIngress{
			IP: lbc.balancerIP,
		})
		if _, err := ingClient.UpdateStatus(currIng); err != nil {
			lbc.recorder.Eventf(currIng, api.EventTypeWarning, "update", "error: %v", err)
			return err
		}

		lbc.recorder.Eventf(currIng, api.EventTypeNormal, "create", "ip: %v", lbc.balancerIP)
	}

	return nil
}

// containsBalancerIP checks for the existence the balancer IP in an ingress
func (lbc *LoadBalancerController) containsBalancerIP(lbings []api.LoadBalancerIngress) bool {

	for _, lbing := range lbings {
		if lbing.IP == lbc.balancerIP {
			return true
		}
	}

	return false
}

// allControllersSynched checks that all controllers have performed the initial list
func (lbc *LoadBalancerController) allControllersSynched() bool {
	return lbc.ingController.HasSynced() &&
		lbc.svcController.HasSynced() &&
		lbc.endpController.HasSynced() &&
		lbc.secretController.HasSynced()
}

func (lbc *LoadBalancerController) getCertificateFromSecret(name string) (*balancer.Certificate, error) {
	secrets := lbc.secretStore.List()
	var secret *api.Secret
	for _, s := range secrets {
		sec := s.(*api.Secret)
		if sec.Name == name {
			secret = sec
		}
	}

	if secret == nil {
		return nil, fmt.Errorf("secret '%v' does not exists", name)
	}

	var pub, priv []byte
	var ok bool

	pub, ok = secret.Data[api.TLSCertKey]
	if !ok {
		return nil, fmt.Errorf("secret %v does not contain a public key", name)
	}

	priv, ok = secret.Data[api.TLSPrivateKeyKey]
	if !ok {
		return nil, fmt.Errorf("secret %v does not contain a private key", name)
	}

	c, err := balancer.NewCertificate(priv, pub, name)
	if err != nil {
		return nil, err
	}
	return c, nil
}
