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
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/workqueue"
)

const (
	reloadQPS            = 10.0
	resyncPeriod         = 10 * time.Second
	ipvsPublicVIP        = "k8s.io/public-vip"
	ipvsPublicVIPNetmask = "k8s.io/public-vip-netmask"
	ipvsPublicVIPRoute   = "k8s.io/public-vip-route"
)

var (
	// keyFunc for endpoints and services.
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc

	// Error used to indicate that a sync is deferred because the controller isn't ready yet
	errDeferredSync = fmt.Errorf("deferring sync till endpoints controller has synced")

	// Define keepalived interface by its ip
	keepalivedIP = flags.String("keepalived-ip", "", `Use given external IP`)
)

type endpoint struct {
	IP   string
	Port int
}

type vport struct {
	Port     int
	Protocol string
	Backends []endpoint
}

type service struct {
	Name      string
	IP        string
	Netmask   int
	Route     string
	ClusterIP string
	Ports     []vport
}

type vroute struct {
	Network string
	Netmask int
	Route   string
	Table   int
	IPs     []string
}

// ipvsControllerController watches the kubernetes api and adds/removes
// services from LVS through ipvsadmin.
type ipvsControllerController struct {
	queue             *workqueue.Type
	client            *unversioned.Client
	epController      *framework.Controller
	svcController     *framework.Controller
	svcLister         cache.StoreToServiceLister
	epLister          cache.StoreToEndpointsLister
	reloadRateLimiter util.RateLimiter
	keepalived        *keepalived
	reloadLock        *sync.Mutex
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (ipvsc *ipvsControllerController) getEndpoints(
	s *api.Service, servicePort *api.ServicePort) (endpoints []endpoint) {
	ep, err := ipvsc.epLister.GetServiceEndpoints(s)
	if err != nil {
		return
	}

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of service traffic for it.
	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {
			var targetPort int
			switch servicePort.TargetPort.Type {
			case intstr.Int:
				if epPort.Port == servicePort.TargetPort.IntValue() {
					targetPort = epPort.Port
				}
			case intstr.String:
				if epPort.Name == servicePort.TargetPort.StrVal {
					targetPort = epPort.Port
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range ss.Addresses {
				endpoints = append(endpoints, endpoint{IP: epAddress.IP, Port: targetPort})
			}
		}
	}
	return
}

// getServices returns a list of services and their endpoints.
func (ipvsc *ipvsControllerController) getServices() []service {
	svcs := []service{}

	services, _ := ipvsc.svcLister.List()
	for _, s := range services.Items {
		annotations := s.GetAnnotations()

		if externalIP, ok := annotations[ipvsPublicVIP]; ok {
			var externalRoute string = ""
			var externalRouteSubnet = 32

			if externalRoute, ok = annotations[ipvsPublicVIPRoute]; ok {
				glog.Infof("Route %v for service %v", externalRoute, s.Name)
			}

			if externalRouteSubnetString, ok := annotations[ipvsPublicVIPNetmask]; ok {
				var err error
				externalRouteSubnet, err = strconv.Atoi(externalRouteSubnetString)
				if err != nil {
					glog.Infof("Cannot parse netmask %v for service %v: %v",
						externalRouteSubnetString, s.Name, err)
					continue
				}
			}

			glog.Infof("Found service: %v", s.Name)

			vports := []vport{}

			for _, servicePort := range s.Spec.Ports {
				glog.Infof("Found service port %v %v", servicePort.Protocol, servicePort.Port)

				ep := ipvsc.getEndpoints(&s, &servicePort)
				if len(ep) == 0 {
					glog.Infof("No endpoints found for service %v, port %+v",
						s.Name, servicePort)
					continue
				}

				glog.Infof("Endpoints %v", ep)

				vport := vport{
					Backends: ep,
					Port:     servicePort.Port,
					Protocol: fmt.Sprintf("%v", servicePort.Protocol),
				}

				vports = append(vports, vport)
			}

			vip := service{
				Name:      fmt.Sprintf("%v/%v", s.Namespace, s.Name),
				IP:        externalIP,
				Netmask:   externalRouteSubnet,
				Route:     externalRoute,
				ClusterIP: s.Spec.ClusterIP,
				Ports:     vports,
			}
			svcs = append(svcs, vip)
		}
	}

	return svcs
}

// getRoutes returns a list of routes.
func (ipvsc *ipvsControllerController) getRoutes(svcs []service) []vroute {
	routes := []vroute{}

	for _, s := range svcs {
		var currentRoute *vroute = nil

		for _, r := range routes {
			if r.Route != s.Route {
				continue
			}

			currentRoute = &r

			if r.Netmask < s.Netmask {
				r.Netmask = s.Netmask
				network, err := calculateNetwork(s.IP, s.Netmask)

				if err != nil {
					glog.Infof("Cannot calculate network for service %v: %v",
						s.Name, err)
					continue
				}

				r.Network = network
			}
		}

		if currentRoute == nil {
			if len(s.Route) > 0 {
				network, err := calculateNetwork(s.IP, s.Netmask)
				if err != nil {
					glog.Infof("Cannot calculate network for service %v: %v",
						s.Name, err)
					continue
				}

				routes = append(routes, vroute{
					Network: network,
					Netmask: s.Netmask,
					Route:   s.Route,
					Table:   len(routes) + 1,
					IPs:     []string{s.IP},
				})
			}
		} else {
			currentRoute.IPs = append(currentRoute.IPs, s.IP)
		}
	}

	return routes
}

// sync all services with the loadbalancer.
func (ipvsc *ipvsControllerController) sync() error {
	ipvsc.reloadRateLimiter.Accept()

	ipvsc.reloadLock.Lock()
	defer ipvsc.reloadLock.Unlock()

	if !ipvsc.epController.HasSynced() || !ipvsc.svcController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
		return errDeferredSync
	}

	services := ipvsc.getServices()
	err := ipvsc.keepalived.WriteCfg(services, ipvsc.getRoutes(services))
	if err != nil {
		return err
	}

	err = ipvsc.keepalived.Reload()
	if err != nil {
		return err
	}

	return nil
}

// worker handles the work queue.
func (ipvsc *ipvsControllerController) worker() {
	for {
		key, _ := ipvsc.queue.Get()
		glog.Infof("Sync triggered by service %v", key)
		if err := ipvsc.sync(); err != nil {
			glog.Infof("Requeuing %v because of error: %v", key, err)
			ipvsc.queue.Add(key)
		}
		ipvsc.queue.Done(key)
	}
}

// newIPVSController creates a new controller from the given config.
func newIPVSController(kubeClient *unversioned.Client, namespace string, useUnicast bool, password string) *ipvsControllerController {
	ipvsc := ipvsControllerController{
		client:            kubeClient,
		queue:             workqueue.New(),
		reloadRateLimiter: util.NewTokenBucketRateLimiter(reloadQPS, int(reloadQPS)),
		reloadLock:        &sync.Mutex{},
	}

	clusterNodes := getClusterNodesIP(kubeClient)

	nodeInfo, err := getNodeInfo(clusterNodes, *keepalivedIP)
	if err != nil {
		glog.Fatalf("Error getting local IP from nodes in the cluster: %v", err)
	}

	neighbors := getNodeNeighbors(nodeInfo, clusterNodes)

	ipvsc.keepalived = &keepalived{
		iface:      nodeInfo.iface,
		ip:         nodeInfo.ip,
		netmask:    nodeInfo.netmask,
		nodes:      clusterNodes,
		neighbors:  neighbors,
		priority:   getNodePriority(nodeInfo.ip, clusterNodes),
		useUnicast: useUnicast,
		password:   password,
	}

	enqueue := func(obj interface{}) {
		key, err := keyFunc(obj)
		if err != nil {
			glog.Infof("Couldn't get key for object %+v: %v", obj, err)
			return
		}

		ipvsc.queue.Add(key)
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

	ipvsc.svcLister.Store, ipvsc.svcController = framework.NewInformer(
		cache.NewListWatchFromClient(
			ipvsc.client, "services", namespace, fields.Everything()),
		&api.Service{}, resyncPeriod, eventHandlers)

	ipvsc.epLister.Store, ipvsc.epController = framework.NewInformer(
		cache.NewListWatchFromClient(
			ipvsc.client, "endpoints", namespace, fields.Everything()),
		&api.Endpoints{}, resyncPeriod, eventHandlers)

	return &ipvsc
}
