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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/experimental"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

var (
	keyFunc          = framework.DeletionHandlingMetaNamespaceKeyFunc
	lbControllerName = "lbcontroller"
)

// loadBalancerController watches the kubernetes api and adds/removes services
// from the loadbalancer, via loadBalancerConfig.
type loadBalancerController struct {
	client         *client.Client
	ingController  *framework.Controller
	nodeController *framework.Controller
	svcController  *framework.Controller
	ingLister      StoreToIngressLister
	nodeLister     cache.StoreToNodeLister
	svcLister      cache.StoreToServiceLister
	clusterManager *ClusterManager
	recorder       record.EventRecorder
	nodeQueue      *taskQueue
	ingQueue       *taskQueue
	tr             *gceTranslator
	stopCh         chan struct{}
	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
}

// NewLoadBalancerController creates a controller for gce loadbalancers.
// - kubeClient: A kubernetes REST client.
// - clusterManager: A ClusterManager capable of creating all cloud resources
//	 required for L7 loadbalancing.
// - resyncPeriod: Watchers relist from the Kubernetes API server this often.
func NewLoadBalancerController(kubeClient *client.Client, clusterManager *ClusterManager, resyncPeriod time.Duration) (*loadBalancerController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(kubeClient.Events(""))

	lbc := loadBalancerController{
		client:         kubeClient,
		clusterManager: clusterManager,
		stopCh:         make(chan struct{}),
		recorder: eventBroadcaster.NewRecorder(
			api.EventSource{Component: "loadbalancer-controller"}),
	}
	lbc.nodeQueue = NewTaskQueue(lbc.syncNodes)
	lbc.ingQueue = NewTaskQueue(lbc.sync)

	// Ingress watch handlers
	pathHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc:    lbc.ingQueue.enqueue,
		DeleteFunc: lbc.ingQueue.enqueue,
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				glog.Infof("Ingress %v changed, syncing",
					cur.(*experimental.Ingress).Name)
			}
			lbc.ingQueue.enqueue(cur)
		},
	}
	lbc.ingLister.Store, lbc.ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(lbc.client),
			WatchFunc: ingressWatchFunc(lbc.client),
		},
		&experimental.Ingress{}, resyncPeriod, pathHandlers)

	// Service watch handlers
	svcHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: lbc.enqueueIngressForService,
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				lbc.enqueueIngressForService(cur)
			}
		},
		// Ingress deletes matter, service deletes don't.
	}

	lbc.svcLister.Store, lbc.svcController = framework.NewInformer(
		cache.NewListWatchFromClient(
			lbc.client, "services", api.NamespaceAll, fields.Everything()),
		&api.Service{}, resyncPeriod, svcHandlers)

	nodeHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc:    lbc.nodeQueue.enqueue,
		DeleteFunc: lbc.nodeQueue.enqueue,
		// Nodes are updated every 10s and we don't care, so no update handler.
	}

	// Node watch handlers
	lbc.nodeLister.Store, lbc.nodeController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return lbc.client.Get().
					Resource("nodes").
					FieldsSelectorParam(fields.Everything()).
					Do().
					Get()
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return lbc.client.Get().
					Prefix("watch").
					Resource("nodes").
					FieldsSelectorParam(fields.Everything()).
					Param("resourceVersion", resourceVersion).Watch()
			},
		},
		&api.Node{}, 0, nodeHandlers)

	lbc.tr = &gceTranslator{&lbc}
	glog.Infof("Created new loadbalancer controller")

	return &lbc, nil
}

func ingressListFunc(c *client.Client) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		return c.Experimental().Ingress(api.NamespaceAll).List(labels.Everything(), fields.Everything())
	}
}

func ingressWatchFunc(c *client.Client) func(rv string) (watch.Interface, error) {
	return func(rv string) (watch.Interface, error) {
		return c.Experimental().Ingress(api.NamespaceAll).Watch(
			labels.Everything(), fields.Everything(), rv)
	}
}

// enqueueIngressForService enqueues all the Ingress' for a Service.
func (lbc *loadBalancerController) enqueueIngressForService(obj interface{}) {
	svc := obj.(*api.Service)
	ings, err := lbc.ingLister.GetServiceIngress(svc)
	if err != nil {
		glog.Infof("Ignoring service %v: %v", svc.Name, err)
		return
	}
	for _, ing := range ings {
		lbc.ingQueue.enqueue(&ing)
	}
}

// Run starts the loadbalancer controller.
func (lbc *loadBalancerController) Run() {
	glog.Infof("Starting loadbalancer controller")
	go lbc.ingController.Run(lbc.stopCh)
	go lbc.nodeController.Run(lbc.stopCh)
	go lbc.svcController.Run(lbc.stopCh)
	go lbc.ingQueue.run(time.Second, lbc.stopCh)
	go lbc.nodeQueue.run(time.Second, lbc.stopCh)
	<-lbc.stopCh
	glog.Infof("Shutting down Loadbalancer Controller")
}

// Stop stops the loadbalancer controller. It deletes shared cluster resources
// only if nothing is using them, leaving everything as-is otherwise.
func (lbc *loadBalancerController) Stop() error {
	// Stop is invoked from the http endpoint.
	lbc.stopLock.Lock()
	defer lbc.stopLock.Unlock()

	// Only try closing the stop channels if we haven't already.
	if !lbc.shutdown {
		close(lbc.stopCh)
		// Stop the workers before invoking cluster shutdown
		glog.Infof("Shutting down controller queues.")
		lbc.ingQueue.shutdown()
		lbc.nodeQueue.shutdown()
		lbc.shutdown = true
	}

	// Deleting shared cluster resources is idempotent.
	glog.Infof("Shutting down cluster manager.")
	return lbc.clusterManager.shutdown()
}

// sync manages the syncing of backends and pathmaps.
func (lbc *loadBalancerController) sync(key string) {
	glog.Infof("Syncing %v", key)

	paths, err := lbc.ingLister.List()
	if err != nil {
		lbc.ingQueue.requeue(key, err)
		return
	}
	nodePorts := lbc.tr.toNodePorts(&paths)
	lbNames := lbc.ingLister.Store.ListKeys()

	// On GC:
	// * Loadbalancers need to get deleted before backends.
	// * Backends are refcounted in a shared pool.
	// * We always want to GC backends even if there was an error in GCing
	//   loadbalancers, because the next Sync could rely on the GC for quota.
	// * There are at least 2 cases for backend GC:
	//   1. The loadbalancer has been deleted.
	//   2. An update to the url map drops the refcount of a backend. This can
	//      happen when an Ingress is updated, if we don't GC after the update
	//      we'll leak the backend.
	defer func() {
		lbErr := lbc.clusterManager.l7Pool.GC(lbNames)
		beErr := lbc.clusterManager.backendPool.GC(nodePorts)
		if lbErr != nil || beErr != nil {
			lbc.ingQueue.requeue(key, fmt.Errorf(
				"Lb GC error: %v, Backend GC error: %v", lbErr, beErr))
		}
		glog.Infof("Finished syncing %v", key)
	}()

	// Create cluster-wide shared resources.
	// TODO: If this turns into a bottleneck, split them out
	// into independent sync pools.
	if err := lbc.clusterManager.l7Pool.Sync(
		lbNames); err != nil {
		lbc.ingQueue.requeue(key, err)
		return
	}
	if err := lbc.clusterManager.backendPool.Sync(
		nodePorts); err != nil {
		lbc.ingQueue.requeue(key, err)
		return
	}

	// Deal with the single loadbalancer that came through the watch
	obj, ingExists, err := lbc.ingLister.Store.GetByKey(key)
	if err != nil {
		lbc.ingQueue.requeue(key, err)
		return
	}
	if !ingExists {
		return
	}
	l7, err := lbc.clusterManager.l7Pool.Get(key)
	if err != nil {
		lbc.ingQueue.requeue(key, err)
		return
	}

	ing := *obj.(*experimental.Ingress)
	if urlMap, err := lbc.tr.toUrlMap(&ing); err != nil {
		lbc.ingQueue.requeue(key, err)
	} else if err := l7.UpdateUrlMap(urlMap); err != nil {
		lbc.ingQueue.requeue(key, err)
	} else if updateLbIp(
		lbc.client.Experimental().Ingress(ing.Namespace),
		ing,
		l7.GetIP()); err != nil {
		lbc.ingQueue.requeue(key, err)
	}
	return
}

// syncNodes manages the syncing of kubernetes nodes to gce instance groups.
// The instancegroups are referenced by loadbalancer backends.
func (lbc *loadBalancerController) syncNodes(key string) {
	kubeNodes, err := lbc.nodeLister.List()
	if err != nil {
		lbc.nodeQueue.requeue(key, err)
		return
	}
	nodeNames := []string{}
	// TODO: delete unhealthy kubernetes nodes from cluster?
	for _, n := range kubeNodes.Items {
		nodeNames = append(nodeNames, n.Name)
	}
	if err := lbc.clusterManager.instancePool.Sync(nodeNames); err != nil {
		lbc.nodeQueue.requeue(key, err)
	}
	return
}
