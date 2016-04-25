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
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/contrib/dns/pkg/providers"
)

var (
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc
)

// dnsController watches the kubernetes api and adds/removes DNS entries
type dnsController struct {
	provider             providers.DNSProvider
	publishServices []string

	client               *client.Client
	ingController        *framework.Controller
	svcController        *framework.Controller
	ingLister            StoreToIngressLister
	svcLister            cache.StoreToServiceLister

	recorder             record.EventRecorder

	syncQueue            *taskQueue

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock             sync.Mutex
	shutdown             bool
	stopCh               chan struct{}
}

// newDNSController creates a controller for DNS
func newDNSController(kubeClient *client.Client,
			resyncPeriod time.Duration,
			provider providers.DNSProvider,
watchNamespace string,
			publishServices []string) (*dnsController, error) {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(kubeClient.Events(""))

	lbc := dnsController{
		provider: provider,
		publishServices: publishServices,
		client:       kubeClient,
		stopCh:       make(chan struct{}),
		recorder:     eventBroadcaster.NewRecorder(api.EventSource{Component: "loadbalancer-controller"}),
	}

	lbc.syncQueue = NewTaskQueue(lbc.sync)

	ingEventHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*extensions.Ingress)
			lbc.recorder.Eventf(addIng, api.EventTypeNormal, "CREATE", fmt.Sprintf("%s/%s", addIng.Namespace, addIng.Name))
			lbc.syncQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			upIng := obj.(*extensions.Ingress)
			lbc.recorder.Eventf(upIng, api.EventTypeNormal, "DELETE", fmt.Sprintf("%s/%s", upIng.Namespace, upIng.Name))
			lbc.syncQueue.enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				upIng := cur.(*extensions.Ingress)
				lbc.recorder.Eventf(upIng, api.EventTypeNormal, "UPDATE", fmt.Sprintf("%s/%s", upIng.Namespace, upIng.Name))
				lbc.syncQueue.enqueue(cur)
			}
		},
	}

	lbc.ingLister.Store, lbc.ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(lbc.client, watchNamespace),
			WatchFunc: ingressWatchFunc(lbc.client, watchNamespace),
		},
		&extensions.Ingress{}, resyncPeriod, ingEventHandler)

	lbc.svcLister.Store, lbc.svcController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(lbc.client, watchNamespace),
			WatchFunc: serviceWatchFunc(lbc.client, watchNamespace),
		},
		&api.Service{}, resyncPeriod, framework.ResourceEventHandlerFuncs{})

	return &lbc, nil
}

func ingressListFunc(c *client.Client, ns string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return c.Extensions().Ingress(ns).List(opts)
	}
}

func ingressWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return c.Extensions().Ingress(ns).Watch(options)
	}
}

func serviceListFunc(c *client.Client, ns string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return c.Services(ns).List(opts)
	}
}

func serviceWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return c.Services(ns).Watch(options)
	}
}

func (lbc *dnsController) controllersInSync() bool {
	return lbc.ingController.HasSynced() && lbc.svcController.HasSynced()
}

func (lbc *dnsController) sync(key string) {
	if !lbc.controllersInSync() {
		lbc.syncQueue.requeue(key, fmt.Errorf("deferring sync till endpoints controller has synced"))
		return
	}

	ings := lbc.ingLister.Store.List()
	names := lbc.buildDNSNames(ings)

	targets := lbc.findTargets()

	err := lbc.provider.EnsureNames(names, targets)
	if err != nil {
		glog.Warningf("error while trying to create names: %v", err)
		// TODO: Add retry logic
	}
}

func (lbc *dnsController) buildDNSNames(data []interface{}) map[string]*providers.DNSName {
	hostnames := make(map[string]*providers.DNSName)

	for _, ingIf := range data {
		ing := ingIf.(*extensions.Ingress)

		for _, rule := range ing.Spec.Rules {
			if _, ok := hostnames[rule.Host]; !ok {
				hostnames[rule.Host] = &providers.DNSName{Name: rule.Host}
			}
		}
	}

	return hostnames
}

func (lbc*dnsController) findTargets() []api.LoadBalancerIngress {
	var ingress []api.LoadBalancerIngress

	for _, externalServiceName := range lbc.publishServices {
		svcObj, svcExists, err := lbc.svcLister.Store.GetByKey(externalServiceName)
		if err != nil {
			// TODO: Add retry logic
			glog.Warningf("error getting service %v: %v", externalServiceName, err)
			continue
		}

		if !svcExists {
			glog.Warningf("service %v was not found", externalServiceName)
			continue
		}

		svc := svcObj.(*api.Service)

		ingress = append(ingress, svc.Status.LoadBalancer.Ingress...)
	}
	return ingress
}

// Stop stops the loadbalancer controller.
func (lbc *dnsController) Stop() error {
	// Stop is invoked from the http endpoint.
	lbc.stopLock.Lock()
	defer lbc.stopLock.Unlock()

	// Only try draining the workqueue if we haven't already.
	if !lbc.shutdown {
		close(lbc.stopCh)
		glog.Infof("shutting down controller queues")
		lbc.shutdown = true
		lbc.syncQueue.shutdown()

		return nil
	}

	return fmt.Errorf("shutdown already in progress")
}

// Run starts the loadbalancer controller.
func (lbc *dnsController) Run() {
	glog.Infof("starting DNS controller")

	go lbc.ingController.Run(lbc.stopCh)
	go lbc.svcController.Run(lbc.stopCh)

	go lbc.syncQueue.run(time.Second, lbc.stopCh)

	<-lbc.stopCh
	glog.Infof("shutting down DNS controller")
}
