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

package controller

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	podutil "k8s.io/kubernetes/pkg/api/pod"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/healthz"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/watch"

	"k8s.io/contrib/ingress/controllers/nginx/nginx"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/config"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/auth"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/authreq"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/cors"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/healthcheck"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/ipwhitelist"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/ratelimit"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/rewrite"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/secureupstream"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/service"
	ssl "k8s.io/contrib/ingress/controllers/nginx/pkg/net/ssl"
)

const (
	defUpstreamName          = "upstream-default-backend"
	defServerName            = "_"
	podStoreSyncedPollPeriod = 1 * time.Second
	rootLocation             = "/"
)

// IngressController ...
type IngressController interface {
	Start()
	Stop() error

	Check() healthz.HealthzChecker
}

// GenericController watches the kubernetes api and adds/removes services from the loadbalancer
type GenericController struct {
	cfg *Configuration

	ingController  *framework.Controller
	endpController *framework.Controller
	svcController  *framework.Controller
	secrController *framework.Controller
	mapController  *framework.Controller

	ingLister  StoreToIngressLister
	svcLister  cache.StoreToServiceLister
	endpLister cache.StoreToEndpointsLister
	secrLister StoreToSecretsLister
	mapLister  StoreToConfigmapLister

	nginx *nginx.Manager

	recorder record.EventRecorder

	syncQueue *taskQueue

	// taskQueue used to update the status of the Ingress rules.
	// this avoids a sync execution in the ResourceEventHandlerFuncs
	ingQueue *taskQueue

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}
}

// Configuration ...
type Configuration struct {
	Client                *client.Client
	ResyncPeriod          time.Duration
	DefaultService        string
	Namespace             string
	NginxConfigMapName    string
	TCPConfigMapName      string
	UDPConfigMapName      string
	DefaultSSLCertificate string
	DefaultHealthzURL     string
}

// NewLoadBalancer creates a controller for nginx loadbalancer
func NewLoadBalancer(config *Configuration) (IngressController, error) {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(config.Client.Events(config.Namespace))

	ic := GenericController{
		cfg:    config,
		stopCh: make(chan struct{}),
		nginx:  nginx.NewManager(config.Client),
		recorder: eventBroadcaster.NewRecorder(api.EventSource{
			Component: "nginx-ingress-controller",
		}),
	}

	ic.syncQueue = NewTaskQueue(ic.sync)
	//ic.ingQueue = NewTaskQueue(ic.updateIngressStatus)

	ingEventHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*extensions.Ingress)
			if !isNGINXIngress(addIng) {
				glog.Infof("Ignoring add for ingress %v based on annotation %v", addIng.Name, ingressClassKey)
				return
			}
			ic.recorder.Eventf(addIng, api.EventTypeNormal, "CREATE", fmt.Sprintf("%s/%s", addIng.Namespace, addIng.Name))
			//ic.ingQueue.enqueue(obj)
			ic.syncQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			delIng := obj.(*extensions.Ingress)
			if !isNGINXIngress(delIng) {
				glog.Infof("Ignoring add for ingress %v based on annotation %v", delIng.Name, ingressClassKey)
				return
			}
			ic.recorder.Eventf(delIng, api.EventTypeNormal, "DELETE", fmt.Sprintf("%s/%s", delIng.Namespace, delIng.Name))
			ic.syncQueue.enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			curIng := cur.(*extensions.Ingress)
			if !isNGINXIngress(curIng) {
				return
			}
			if !reflect.DeepEqual(old, cur) {
				upIng := cur.(*extensions.Ingress)
				ic.recorder.Eventf(upIng, api.EventTypeNormal, "UPDATE", fmt.Sprintf("%s/%s", upIng.Namespace, upIng.Name))
				//ic.ingQueue.enqueue(cur)
				ic.syncQueue.enqueue(cur)
			}
		},
	}

	secrEventHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addSecr := obj.(*api.Secret)
			if ic.secrReferenced(addSecr.Namespace, addSecr.Name) {
				ic.recorder.Eventf(addSecr, api.EventTypeNormal, "CREATE", fmt.Sprintf("%s/%s", addSecr.Namespace, addSecr.Name))
				ic.syncQueue.enqueue(obj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			delSecr := obj.(*api.Secret)
			if ic.secrReferenced(delSecr.Namespace, delSecr.Name) {
				ic.recorder.Eventf(delSecr, api.EventTypeNormal, "DELETE", fmt.Sprintf("%s/%s", delSecr.Namespace, delSecr.Name))
				ic.syncQueue.enqueue(obj)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				upSecr := cur.(*api.Secret)
				if ic.secrReferenced(upSecr.Namespace, upSecr.Name) {
					ic.recorder.Eventf(upSecr, api.EventTypeNormal, "UPDATE", fmt.Sprintf("%s/%s", upSecr.Namespace, upSecr.Name))
					ic.syncQueue.enqueue(cur)
				}
			}
		},
	}

	eventHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ic.syncQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			ic.syncQueue.enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				ic.syncQueue.enqueue(cur)
			}
		},
	}

	mapEventHandler := framework.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				upCmap := cur.(*api.ConfigMap)
				mapKey := fmt.Sprintf("%s/%s", upCmap.Namespace, upCmap.Name)
				// updates to configuration configmaps can trigger an update
				if mapKey == ic.cfg.NginxConfigMapName || mapKey == ic.cfg.TCPConfigMapName || mapKey == ic.cfg.UDPConfigMapName {
					ic.recorder.Eventf(upCmap, api.EventTypeNormal, "UPDATE", mapKey)
					ic.syncQueue.enqueue(cur)
				}
			}
		},
	}

	ic.ingLister.Store, ic.ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ic.ingressListFunc(),
			WatchFunc: ic.ingressWatchFunc(),
		},
		&extensions.Ingress{}, ic.cfg.ResyncPeriod, ingEventHandler)

	ic.endpLister.Store, ic.endpController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ic.endpointsListFunc(),
			WatchFunc: ic.endpointsWatchFunc(),
		},
		&api.Endpoints{}, ic.cfg.ResyncPeriod, eventHandler)

	ic.svcLister.Store, ic.svcController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ic.serviceListFunc(),
			WatchFunc: ic.serviceWatchFunc(),
		},
		&api.Service{}, ic.cfg.ResyncPeriod, framework.ResourceEventHandlerFuncs{})

	ic.secrLister.Store, ic.secrController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ic.secretsListFunc(),
			WatchFunc: ic.secretsWatchFunc(),
		},
		&api.Secret{}, ic.cfg.ResyncPeriod, secrEventHandler)

	ic.mapLister.Store, ic.mapController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ic.mapListFunc(),
			WatchFunc: ic.mapWatchFunc(),
		},
		&api.ConfigMap{}, ic.cfg.ResyncPeriod, mapEventHandler)

	return ic, nil
}

func (ic *GenericController) ingressListFunc() func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return ic.cfg.Client.Extensions().Ingress(ic.cfg.Namespace).List(opts)
	}
}

func (ic *GenericController) ingressWatchFunc() func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return ic.cfg.Client.Extensions().Ingress(ic.cfg.Namespace).Watch(options)
	}
}

func (ic *GenericController) serviceListFunc() func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return ic.cfg.Client.Services(ic.cfg.Namespace).List(opts)
	}
}

func (ic *GenericController) serviceWatchFunc() func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return ic.cfg.Client.Services(ic.cfg.Namespace).Watch(options)
	}
}

func (ic *GenericController) endpointsListFunc() func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return ic.cfg.Client.Endpoints(ic.cfg.Namespace).List(opts)
	}
}

func (ic *GenericController) endpointsWatchFunc() func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return ic.cfg.Client.Endpoints(ic.cfg.Namespace).Watch(options)
	}
}

func (ic *GenericController) secretsListFunc() func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return ic.cfg.Client.Secrets(ic.cfg.Namespace).List(opts)
	}
}

func (ic *GenericController) secretsWatchFunc() func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return ic.cfg.Client.Secrets(ic.cfg.Namespace).Watch(options)
	}
}

func (ic *GenericController) mapListFunc() func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return ic.cfg.Client.ConfigMaps(ic.cfg.Namespace).List(opts)
	}
}

func (ic *GenericController) mapWatchFunc() func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return ic.cfg.Client.ConfigMaps(ic.cfg.Namespace).Watch(options)
	}
}

func (ic *GenericController) controllersInSync() bool {
	return ic.ingController.HasSynced() &&
		ic.svcController.HasSynced() &&
		ic.endpController.HasSynced() &&
		ic.secrController.HasSynced() &&
		ic.mapController.HasSynced()
}

func (ic *GenericController) getConfigMap(ns, name string) (*api.ConfigMap, error) {
	// TODO: check why ic.mapLister.Store.GetByKey(mapKey) is not stable (random content)
	return ic.cfg.Client.ConfigMaps(ns).Get(name)
}

func (ic *GenericController) getTCPConfigMap(ns, name string) (*api.ConfigMap, error) {
	return ic.getConfigMap(ns, name)
}

func (ic *GenericController) getUDPConfigMap(ns, name string) (*api.ConfigMap, error) {
	return ic.getConfigMap(ns, name)
}

// Check returns if the healthz endpoint is returning ok (status code 200)
func (ic GenericController) Check() healthz.HealthzChecker {
	return *ic.nginx
}

// checkSvcForUpdate verifies if one of the running pods for a service contains
// named port. If the annotation in the service does not exists or is not equals
// to the port mapping obtained from the pod the service must be updated to reflect
// the current state
func (ic *GenericController) checkSvcForUpdate(svc *api.Service) error {
	// get the pods associated with the service
	// TODO: switch this to a watch
	pods, err := ic.cfg.Client.Pods(svc.Namespace).List(api.ListOptions{
		LabelSelector: labels.Set(svc.Spec.Selector).AsSelector(),
	})

	if err != nil {
		fmt.Errorf("error searching service pods %v/%v: %v", svc.Namespace, svc.Name, err)
	}

	if len(pods.Items) == 0 {
		return nil
	}

	// we need to check only one pod searching for named ports
	pod := &pods.Items[0]
	glog.V(4).Infof("checking pod %v/%v for named port information", pod.Namespace, pod.Name)
	for i := range svc.Spec.Ports {
		servicePort := &svc.Spec.Ports[i]

		_, err := strconv.Atoi(servicePort.TargetPort.StrVal)
		if err != nil {
			portNum, err := podutil.FindPort(pod, servicePort)
			if err != nil {
				glog.V(4).Infof("failed to find port for service %s/%s: %v", portNum, svc.Namespace, svc.Name, err)
				continue
			}

			if servicePort.TargetPort.StrVal == "" {
				continue
			}

			//namedPorts[servicePort.TargetPort.StrVal] = fmt.Sprintf("%v", portNum)
		}
	}

	if svc.ObjectMeta.Annotations == nil {
		svc.ObjectMeta.Annotations = map[string]string{}
	}
	/*
		curNamedPort := svc.ObjectMeta.Annotations[namedPortAnnotation]
		if len(namedPorts) > 0 && !reflect.DeepEqual(curNamedPort, namedPorts) {
			data, _ := json.Marshal(namedPorts)

			newSvc, err := ic.cfg.Client.Services(svc.Namespace).Get(svc.Name)
			if err != nil {
				return namedPorts, fmt.Errorf("error getting service %v/%v: %v", svc.Namespace, svc.Name, err)
			}

			if newSvc.ObjectMeta.Annotations == nil {
				newSvc.ObjectMeta.Annotations = map[string]string{}
			}

			newSvc.ObjectMeta.Annotations[namedPortAnnotation] = string(data)
			glog.Infof("updating service %v with new named port mappings", svc.Name)
			_, err = ic.cfg.Client.Services(svc.Namespace).Update(newSvc)
			if err != nil {
				return fmt.Errorf("error syncing service %v/%v: %v", svc.Namespace, svc.Name, err)
			}

			return newSvc.ObjectMeta.Annotations, nil
		}*/

	return nil
}

func (ic *GenericController) sync(key string) error {
	if !ic.controllersInSync() {
		time.Sleep(podStoreSyncedPollPeriod)
		return fmt.Errorf("deferring sync till endpoints controller has synced")
	}

	// by default no custom configuration configmap
	cfg := &api.ConfigMap{}

	if ic.cfg.NginxConfigMapName != "" {
		// Search for custom configmap (defined in main args)
		var err error
		ns, name, _ := parseNsName(ic.cfg.NginxConfigMapName)
		cfg, err = ic.getConfigMap(ns, name)
		if err != nil {
			return fmt.Errorf("unexpected error searching configmap %v: %v", ic.cfg.NginxConfigMapName, err)
		}
	}

	ngxConfig := ic.nginx.ReadConfig(cfg)
	ngxConfig.HealthzURL = ic.cfg.DefaultHealthzURL

	ings := ic.ingLister.Store.List()
	upstreams, servers := ic.getUpstreamServers(ngxConfig, ings)

	return ic.nginx.CheckAndReload(ngxConfig, ingress.Configuration{
		Upstreams:    upstreams,
		Servers:      servers,
		TCPUpstreams: ic.getTCPServices(),
		UDPUpstreams: ic.getUDPServices(),
	})
}

func (ic *GenericController) getTCPServices() []*ingress.Location {
	if ic.cfg.TCPConfigMapName == "" {
		// no configmap for TCP services
		return []*ingress.Location{}
	}

	ns, name, err := parseNsName(ic.cfg.TCPConfigMapName)
	if err != nil {
		glog.Warningf("%v", err)
		return []*ingress.Location{}
	}
	tcpMap, err := ic.getTCPConfigMap(ns, name)
	if err != nil {
		glog.V(3).Infof("no configured tcp services found: %v", err)
		return []*ingress.Location{}
	}

	return ic.getStreamServices(tcpMap.Data, api.ProtocolTCP)
}

func (ic *GenericController) getUDPServices() []*ingress.Location {
	if ic.cfg.UDPConfigMapName == "" {
		// no configmap for TCP services
		return []*ingress.Location{}
	}

	ns, name, err := parseNsName(ic.cfg.UDPConfigMapName)
	if err != nil {
		glog.Warningf("%v", err)
		return []*ingress.Location{}
	}
	tcpMap, err := ic.getUDPConfigMap(ns, name)
	if err != nil {
		glog.V(3).Infof("no configured tcp services found: %v", err)
		return []*ingress.Location{}
	}

	return ic.getStreamServices(tcpMap.Data, api.ProtocolUDP)
}

func (ic *GenericController) getStreamServices(data map[string]string, proto api.Protocol) []*ingress.Location {
	var svcs []*ingress.Location
	// k -> port to expose in nginx
	// v -> <namespace>/<service name>:<port from service to be used>
	for k, v := range data {
		port, err := strconv.Atoi(k)
		if err != nil {
			glog.Warningf("%v is not valid as a TCP port", k)
			continue
		}

		// this ports are required for NGINX
		if k == "80" || k == "443" || k == "8181" {
			glog.Warningf("port %v cannot be used for TCP or UDP services. Is reserved for NGINX", k)
			continue
		}

		nsSvcPort := strings.Split(v, ":")
		if len(nsSvcPort) != 2 {
			glog.Warningf("invalid format (namespace/name:port) '%v'", k)
			continue
		}

		nsName := nsSvcPort[0]
		svcPort := nsSvcPort[1]

		svcNs, svcName, err := parseNsName(nsName)
		if err != nil {
			glog.Warningf("%v", err)
			continue
		}

		svcObj, svcExists, err := ic.svcLister.Store.GetByKey(nsName)
		if err != nil {
			glog.Warningf("error getting service %v: %v", nsName, err)
			continue
		}

		if !svcExists {
			glog.Warningf("service %v was not found", nsName)
			continue
		}

		svc := svcObj.(*api.Service)

		var endps []ingress.UpstreamServer
		targetPort, err := strconv.Atoi(svcPort)
		if err != nil {
			for _, sp := range svc.Spec.Ports {
				if sp.Name == svcPort {
					endps = ic.getEndpoints(svc, sp.TargetPort, proto, &healthcheck.Upstream{})
					break
				}
			}
		} else {
			// we need to use the TargetPort (where the endpoints are running)
			for _, sp := range svc.Spec.Ports {
				if sp.Port == int32(targetPort) {
					endps = ic.getEndpoints(svc, sp.TargetPort, proto, &healthcheck.Upstream{})
					break
				}
			}
		}

		// tcp upstreams cannot contain empty upstreams and there is no
		// default backend equivalent for TCP
		if len(endps) == 0 {
			glog.Warningf("service %v/%v does not have any active endpoints", svcNs, svcName)
			continue
		}

		svcs = append(svcs, &ingress.Location{
			Path: k,
			Upstream: ingress.Upstream{
				Name:     fmt.Sprintf("%v-%v-%v", svcNs, svcName, port),
				Backends: endps,
			},
		})
	}

	return svcs
}

// getDefaultUpstream returns an NGINX upstream associated with the
// default backend service. In case of error retrieving information
// configure the upstream to return http code 503.
func (ic *GenericController) getDefaultUpstream() *ingress.Upstream {
	upstream := &ingress.Upstream{
		Name: defUpstreamName,
	}
	svcKey := ic.cfg.DefaultService
	svcObj, svcExists, err := ic.svcLister.Store.GetByKey(svcKey)
	if err != nil {
		glog.Warningf("unexpected error searching the default backend %v: %v", ic.cfg.DefaultService, err)
		upstream.Backends = append(upstream.Backends, nginx.NewDefaultServer())
		return upstream
	}

	if !svcExists {
		glog.Warningf("service %v does not exists", svcKey)
		upstream.Backends = append(upstream.Backends, nginx.NewDefaultServer())
		return upstream
	}

	svc := svcObj.(*api.Service)

	endps := ic.getEndpoints(svc, svc.Spec.Ports[0].TargetPort, api.ProtocolTCP, &healthcheck.Upstream{})
	if len(endps) == 0 {
		glog.Warningf("service %v does not have any active endpoints", svcKey)
		endps = []ingress.UpstreamServer{nginx.NewDefaultServer()}
	}

	upstream.Backends = append(upstream.Backends, endps...)

	return upstream
}

// getUpstreamServers returns a list of Upstream and Server to be used in NGINX.
// An upstream can be used in multiple servers if the namespace, service name and port are the same
func (ic *GenericController) getUpstreamServers(ngxCfg config.Configuration, data []interface{}) ([]*ingress.Upstream, []*ingress.Server) {
	upstreams := ic.createUpstreams(ngxCfg, data)
	servers := ic.createServers(data, upstreams)

	for _, ingIf := range data {
		ing := ingIf.(*extensions.Ingress)

		nginxAuth, err := auth.ParseAnnotations(ic.cfg.Client, ing, auth.DefAuthDirectory)
		glog.V(3).Infof("nginx auth %v", nginxAuth)
		if err != nil {
			glog.V(3).Infof("error reading authentication in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		rl, err := ratelimit.ParseAnnotations(ing)
		glog.V(3).Infof("nginx rate limit %v", rl)
		if err != nil {
			glog.V(3).Infof("error reading rate limit annotation in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		secUpstream, err := secureupstream.ParseAnnotations(ing)
		if err != nil {
			glog.V(3).Infof("error reading secure upstream in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		locRew, err := rewrite.ParseAnnotations(ngxCfg, ing)
		if err != nil {
			glog.V(3).Infof("error parsing rewrite annotations for Ingress rule %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		wl, err := ipwhitelist.ParseAnnotations(ngxCfg.WhitelistSourceRange, ing)
		glog.V(3).Infof("nginx white list %v", wl)
		if err != nil {
			glog.V(3).Infof("error reading white list annotation in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		eCORS, err := cors.ParseAnnotations(ing)
		if err != nil {
			glog.V(3).Infof("error reading CORS annotation in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		ra, err := authreq.ParseAnnotations(ing)
		glog.V(3).Infof("nginx auth request %v", ra)
		if err != nil {
			glog.V(3).Infof("error reading auth request annotation in Ingress %v/%v: %v", ing.GetNamespace(), ing.GetName(), err)
		}

		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = defServerName
			}
			server := servers[host]
			if server == nil {
				server = servers[defServerName]
			}

			if rule.HTTP == nil && host != defServerName {
				// no rules, host is not default server.
				// check if Ingress rules contains Backend and replace default backend
				defBackend := fmt.Sprintf("default-backend-%v-%v-%v", ing.GetNamespace(), ing.Spec.Backend.ServiceName, ing.Spec.Backend.ServicePort.String())
				ups := upstreams[defBackend]
				for _, loc := range server.Locations {
					loc.Upstream = *ups
				}
				continue
			}

			for _, path := range rule.HTTP.Paths {
				upsName := fmt.Sprintf("%v-%v-%v", ing.GetNamespace(), path.Backend.ServiceName, path.Backend.ServicePort.String())
				ups := upstreams[upsName]

				// we need to check if the upstream contains the default backend
				if isDefaultUpstream(ups) && ing.Spec.Backend != nil {
					defBackend := fmt.Sprintf("default-backend-%v-%v-%v", ing.GetNamespace(), ing.Spec.Backend.ServiceName, ing.Spec.Backend.ServicePort.String())
					if defUps, ok := upstreams[defBackend]; ok {
						ups = defUps
					}
				}

				nginxPath := path.Path
				// if there's no path defined we assume /
				// in NGINX / == /*
				if nginxPath == "" {
					ic.recorder.Eventf(ing, api.EventTypeWarning, "MAPPING",
						"Ingress rule '%v/%v' contains no path definition. Assuming /",
						ing.GetNamespace(), ing.GetName())
					nginxPath = rootLocation
				}

				// Validate that there is no previous rule for the same host and path.
				addLoc := true
				for _, loc := range server.Locations {
					if loc.Path == rootLocation && nginxPath == rootLocation && loc.IsDefBackend {
						loc.Upstream = *ups
						loc.BasicDigestAuth = *nginxAuth
						loc.RateLimit = *rl
						loc.Redirect = *locRew
						loc.SecureUpstream = secUpstream
						loc.Whitelist = *wl
						loc.IsDefBackend = false
						loc.Upstream = *ups
						loc.EnableCORS = eCORS
						loc.ExternalAuth = ra

						addLoc = false
						continue
					}

					if loc.Path == nginxPath {
						ic.recorder.Eventf(ing, api.EventTypeWarning, "MAPPING",
							"Path '%v' already defined in another Ingress rule", nginxPath)
						addLoc = false
						break
					}
				}

				if addLoc {
					server.Locations = append(server.Locations, &ingress.Location{
						Path:            nginxPath,
						Upstream:        *ups,
						BasicDigestAuth: *nginxAuth,
						RateLimit:       *rl,
						Redirect:        *locRew,
						SecureUpstream:  secUpstream,
						Whitelist:       *wl,
						EnableCORS:      eCORS,
						ExternalAuth:    ra,
					})
				}
			}
		}
	}

	// TODO: find a way to make this more readable
	// The structs must be ordered to always generate the same file
	// if the content does not change.
	aUpstreams := make([]*ingress.Upstream, 0, len(upstreams))
	for _, value := range upstreams {
		if len(value.Backends) == 0 {
			glog.Warningf("upstream %v does not have any active endpoints. Using default backend", value.Name)
			value.Backends = append(value.Backends, nginx.NewDefaultServer())
		}
		sort.Sort(ingress.UpstreamServerByAddrPort(value.Backends))
		aUpstreams = append(aUpstreams, value)
	}
	sort.Sort(ingress.UpstreamByNameServers(aUpstreams))

	aServers := make([]*ingress.Server, 0, len(servers))
	for _, value := range servers {
		sort.Sort(ingress.LocationByPath(value.Locations))
		aServers = append(aServers, value)
	}
	sort.Sort(ingress.ServerByName(aServers))

	return aUpstreams, aServers
}

// createUpstreams creates the NGINX upstreams for each service referenced in
// Ingress rules. The servers inside the upstream are endpoints.
func (ic *GenericController) createUpstreams(ngxCfg config.Configuration, data []interface{}) map[string]*ingress.Upstream {
	upstreams := make(map[string]*ingress.Upstream)
	upstreams[defUpstreamName] = ic.getDefaultUpstream()

	for _, ingIf := range data {
		ing := ingIf.(*extensions.Ingress)

		hz := healthcheck.ParseAnnotations(ngxCfg, ing)

		var defBackend string
		if ing.Spec.Backend != nil {
			defBackend = fmt.Sprintf("default-backend-%v-%v-%v", ing.GetNamespace(), ing.Spec.Backend.ServiceName, ing.Spec.Backend.ServicePort.String())
			glog.V(3).Infof("creating upstream %v", defBackend)
			upstreams[defBackend] = nginx.NewUpstream(defBackend)

			svcKey := fmt.Sprintf("%v/%v", ing.GetNamespace(), ing.Spec.Backend.ServiceName)
			endps, err := ic.getSvcEndpoints(svcKey, ing.Spec.Backend.ServicePort.String(), hz)
			upstreams[defBackend].Backends = append(upstreams[defBackend].Backends, endps...)
			if err != nil {
				glog.Warningf("error creating upstream %v: %v", defBackend, err)
			}
		}

		for _, rule := range ing.Spec.Rules {
			if rule.IngressRuleValue.HTTP == nil {
				continue
			}

			for _, path := range rule.HTTP.Paths {
				name := fmt.Sprintf("%v-%v-%v", ing.GetNamespace(), path.Backend.ServiceName, path.Backend.ServicePort.String())
				if _, ok := upstreams[name]; ok {
					continue
				}

				glog.V(3).Infof("creating upstream %v", name)
				upstreams[name] = nginx.NewUpstream(name)

				svcKey := fmt.Sprintf("%v/%v", ing.GetNamespace(), path.Backend.ServiceName)
				endp, err := ic.getSvcEndpoints(svcKey, path.Backend.ServicePort.String(), hz)
				if err != nil {
					glog.Warningf("error obtaining service endpoints: %v", err)
					continue
				}
				upstreams[name].Backends = endp
			}
		}
	}

	return upstreams
}

func (ic *GenericController) getSvcEndpoints(svcKey, backendPort string,
	hz *healthcheck.Upstream) ([]ingress.UpstreamServer, error) {
	svcObj, svcExists, err := ic.svcLister.Store.GetByKey(svcKey)

	var upstreams []ingress.UpstreamServer
	if err != nil {
		return upstreams, fmt.Errorf("error getting service %v from the cache: %v", svcKey, err)
	}

	if !svcExists {
		err = fmt.Errorf("service %v does not exists", svcKey)
		return upstreams, err
	}

	svc := svcObj.(*api.Service)
	glog.V(3).Infof("obtaining port information for service %v", svcKey)
	for _, servicePort := range svc.Spec.Ports {
		// targetPort could be a string, use the name or the port (int)
		if strconv.Itoa(int(servicePort.Port)) == backendPort ||
			servicePort.TargetPort.String() == backendPort ||
			servicePort.Name == backendPort {

			endps := ic.getEndpoints(svc, servicePort.TargetPort, api.ProtocolTCP, hz)
			if len(endps) == 0 {
				glog.Warningf("service %v does not have any active endpoints", svcKey)
			}

			upstreams = append(upstreams, endps...)
			break
		}
	}

	return upstreams, nil
}

func (ic *GenericController) createServers(data []interface{}, upstreams map[string]*ingress.Upstream) map[string]*ingress.Server {
	servers := make(map[string]*ingress.Server)

	pems := ic.getPemsFromIngress(data)

	var ngxCert ingress.SSLCert
	var err error

	if ic.cfg.DefaultSSLCertificate == "" {
		// use system certificated generated at image build time
		cert, key := getFakeSSLCert()
		ngxCert, err = ssl.AddOrUpdateCertAndKey("system-snake-oil-certificate", cert, key)
	} else {
		ngxCert, err = ic.getPemCertificate(ic.cfg.DefaultSSLCertificate)
	}

	locs := []*ingress.Location{}
	locs = append(locs, &ingress.Location{
		Path:         rootLocation,
		IsDefBackend: true,
		Upstream:     *ic.getDefaultUpstream(),
	})
	servers[defServerName] = &ingress.Server{Name: defServerName, Locations: locs}

	if err == nil {
		pems[defServerName] = ngxCert
		servers[defServerName].SSL = true
		servers[defServerName].SSLCertificate = ngxCert.PemFileName
		servers[defServerName].SSLCertificateKey = ngxCert.PemFileName
		servers[defServerName].SSLPemChecksum = ngxCert.PemSHA
	} else {
		glog.Warningf("unexpected error reading default SSL certificate: %v", err)
	}

	for _, ingIf := range data {
		ing := ingIf.(*extensions.Ingress)

		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = defServerName
			}

			if _, ok := servers[host]; ok {
				glog.V(3).Infof("rule %v/%v uses a host already defined. Skipping server creation", ing.GetNamespace(), ing.GetName())
			} else {
				locs := []*ingress.Location{}
				loc := &ingress.Location{
					Path:         rootLocation,
					IsDefBackend: true,
					Upstream:     *ic.getDefaultUpstream(),
				}

				if ing.Spec.Backend != nil {
					defUpstream := fmt.Sprintf("default-backend-%v-%v-%v", ing.GetNamespace(), ing.Spec.Backend.ServiceName, ing.Spec.Backend.ServicePort.String())
					if backendUpstream, ok := upstreams[defUpstream]; ok {
						if host == "" || host == defServerName {
							ic.recorder.Eventf(ing, api.EventTypeWarning, "MAPPING", "error: rules with Spec.Backend are allowed with hostnames")
						} else {
							loc.Upstream = *backendUpstream
						}
					}
				}

				locs = append(locs, loc)
				servers[host] = &ingress.Server{Name: host, Locations: locs}
			}

			if ngxCert, ok := pems[host]; ok {
				server := servers[host]
				server.SSL = true
				server.SSLCertificate = ngxCert.PemFileName
				server.SSLCertificateKey = ngxCert.PemFileName
				server.SSLPemChecksum = ngxCert.PemSHA
			}
		}
	}

	return servers
}

func (ic *GenericController) getPemsFromIngress(data []interface{}) map[string]ingress.SSLCert {
	pems := make(map[string]ingress.SSLCert)

	for _, ingIf := range data {
		ing := ingIf.(*extensions.Ingress)
		for _, tls := range ing.Spec.TLS {
			secretName := tls.SecretName
			secretKey := fmt.Sprintf("%s/%s", ing.Namespace, secretName)

			ngxCert, err := ic.getPemCertificate(secretKey)
			if err != nil {
				glog.Warningf("%v", err)
				continue
			}

			for _, host := range tls.Hosts {
				if isHostValid(host, ngxCert.CN) {
					pems[host] = ngxCert
				} else {
					glog.Warningf("SSL Certificate stored in secret %v is not valid for the host %v defined in the Ingress rule %v", secretName, host, ing.Name)
				}
			}
		}
	}

	return pems
}

func (ic *GenericController) getPemCertificate(secretName string) (ingress.SSLCert, error) {
	secretInterface, exists, err := ic.secrLister.Store.GetByKey(secretName)
	if err != nil {
		return ingress.SSLCert{}, fmt.Errorf("Error retriveing secret %v: %v", secretName, err)
	}
	if !exists {
		return ingress.SSLCert{}, fmt.Errorf("Secret %v does not exists", secretName)
	}

	secret := secretInterface.(*api.Secret)
	cert, ok := secret.Data[api.TLSCertKey]
	if !ok {
		return ingress.SSLCert{}, fmt.Errorf("Secret %v has no private key", secretName)
	}
	key, ok := secret.Data[api.TLSPrivateKeyKey]
	if !ok {
		return ingress.SSLCert{}, fmt.Errorf("Secret %v has no cert", secretName)
	}

	nsSecName := strings.Replace(secretName, "/", "-", -1)
	return ssl.AddOrUpdateCertAndKey(nsSecName, string(cert), string(key))
}

// check if secret is referenced in this controller's config
func (ic *GenericController) secrReferenced(namespace string, name string) bool {
	for _, ingIf := range ic.ingLister.Store.List() {
		ing := ingIf.(*extensions.Ingress)
		if ing.Namespace != namespace {
			continue
		}
		for _, tls := range ing.Spec.TLS {
			if tls.SecretName == name {
				return true
			}
		}
	}
	return false
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (ic *GenericController) getEndpoints(
	s *api.Service,
	servicePort intstr.IntOrString,
	proto api.Protocol,
	hz *healthcheck.Upstream) []ingress.UpstreamServer {
	glog.V(3).Infof("getting endpoints for service %v/%v and port %v", s.Namespace, s.Name, servicePort.String())
	ep, err := ic.endpLister.GetServiceEndpoints(s)
	if err != nil {
		glog.Warningf("unexpected error obtaining service endpoints: %v", err)
		return []ingress.UpstreamServer{}
	}

	upsServers := []ingress.UpstreamServer{}

	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {

			if !reflect.DeepEqual(epPort.Protocol, proto) {
				continue
			}

			var targetPort int32

			switch servicePort.Type {
			case intstr.Int:
				if int(epPort.Port) == servicePort.IntValue() {
					targetPort = epPort.Port
				}
			case intstr.String:
				port, err := service.GetPortMapping(servicePort.StrVal, s)
				if err == nil {
					targetPort = port
				} else {
					glog.Warningf("error mapping service port: %v", err)
					err := ic.checkSvcForUpdate(s)
					if err != nil {
						glog.Warningf("error mapping service ports: %v", err)
						continue
					}

					port, err := service.GetPortMapping(servicePort.StrVal, s)
					if err == nil {
						targetPort = port
					}
				}
			}

			// check for invalid port value
			if targetPort == -1 {
				continue
			}

			for _, epAddress := range ss.Addresses {
				ups := ingress.UpstreamServer{
					Address:     epAddress.IP,
					Port:        fmt.Sprintf("%v", targetPort),
					MaxFails:    hz.MaxFails,
					FailTimeout: hz.FailTimeout,
				}
				upsServers = append(upsServers, ups)
			}
		}
	}

	glog.V(3).Infof("endpoints found: %v", upsServers)
	return upsServers
}

// Stop stops the loadbalancer controller.
func (ic GenericController) Stop() error {
	// Stop is invoked from the http endpoint.
	ic.stopLock.Lock()
	defer ic.stopLock.Unlock()

	// Only try draining the workqueue if we haven't already.
	if !ic.shutdown {
		ic.shutdown = true
		close(ic.stopCh)

		//ings := ic.ingLister.Store.List()
		//glog.Infof("removing IP address %v from ingress rules", ic.podInfo.NodeIP)
		//ic.removeFromIngress(ings)

		glog.Infof("Shutting down controller queues.")
		ic.syncQueue.shutdown()
		//ic.ingQueue.shutdown()

		return nil
	}

	return fmt.Errorf("shutdown already in progress")
}

// Start starts the loadbalancer controller.
func (ic GenericController) Start() {
	glog.Infof("starting NGINX loadbalancer controller")
	go ic.nginx.Start()

	go ic.ingController.Run(ic.stopCh)
	go ic.endpController.Run(ic.stopCh)
	go ic.svcController.Run(ic.stopCh)
	go ic.secrController.Run(ic.stopCh)
	go ic.mapController.Run(ic.stopCh)

	go ic.syncQueue.run(time.Second, ic.stopCh)
	//go ic.ingQueue.run(time.Second, ic.stopCh)

	<-ic.stopCh
}
