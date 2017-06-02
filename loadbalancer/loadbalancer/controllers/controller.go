/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/contrib/loadbalancer/loadbalancer/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// LoadBalancerController watches Kubernetes API for ConfigMap and node changes
// and reconfigures backend when needed
type LoadBalancerController struct {
	client              *client.Client
	configMapController *framework.Controller
	configMapLister     StoreToConfigMapLister
	nodeController      *framework.Controller
	nodeLister          cache.StoreToNodeLister
	configMapQueue      *taskQueue
	stopCh              chan struct{}
	backendController   backend.BackendController
}

// StoreToConfigMapLister makes a Store that lists ConfigMap.
type StoreToConfigMapLister struct {
	cache.Store
}

var keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc

// NewLoadBalancerController creates a controller
func NewLoadBalancerController(kubeClient *client.Client, resyncPeriod time.Duration, namespace string, controller backend.BackendController, configMapLabelKey, configMapLabelValue string) (*LoadBalancerController, error) {
	lbController := LoadBalancerController{
		client:            kubeClient,
		stopCh:            make(chan struct{}),
		backendController: controller,
	}
	lbController.configMapQueue = NewTaskQueue(lbController.syncConfigMap)

	configMapHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lbController.configMapQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			deletedConfigMap := obj.(*api.ConfigMap)
			go lbController.backendController.HandleConfigMapDelete(deletedConfigMap)
		},
		UpdateFunc: func(old, cur interface{}) {
			curCM := old.(*api.ConfigMap).Data
			oldCM := cur.(*api.ConfigMap).Data
			if !configmapsEqual(oldCM, curCM) {
				lbController.configMapQueue.enqueue(cur)
			}
		},
	}
	lbController.configMapLister.Store, lbController.configMapController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  configMapListFunc(kubeClient, namespace, configMapLabelKey, configMapLabelValue),
			WatchFunc: configMapWatchFunc(kubeClient, namespace, configMapLabelKey, configMapLabelValue),
		},
		&api.ConfigMap{}, resyncPeriod, configMapHandlers)

	nodeHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addNode := obj.(*api.Node)
			if utils.NodeReady(*addNode) {
				go lbController.backendController.HandleNodeCreate(addNode)
			}
		},
		DeleteFunc: func(obj interface{}) {
			delNode := obj.(*api.Node)
			if utils.NodeReady(*delNode) {
				go lbController.backendController.HandleNodeDelete(delNode)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			// Only sync nodes when they are in READY state and have their IPs changed
			curNode := cur.(*api.Node)
			if utils.NodeReady(*curNode) {
				oldNode := old.(*api.Node)
				oldNodeIP, _ := utils.GetNodeHostIP(*oldNode)
				curNodeIP, _ := utils.GetNodeHostIP(*curNode)
				if oldNodeIP == nil {
					glog.Infof("Updated node %v. IP set to %v. Syncing", curNode.Name, *curNodeIP)
					go lbController.backendController.HandleNodeCreate(curNode)
				} else if *oldNodeIP != *curNodeIP {
					glog.Infof("Updated node %v. IP changed from %v to %v. Syncing", curNode.Name, *oldNodeIP, *curNodeIP)
					go lbController.backendController.HandleNodeUpdate(oldNode, curNode)
				}
			}
		},
	}

	lbController.nodeLister.Store, lbController.nodeController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(opts api.ListOptions) (runtime.Object, error) {
				return lbController.client.Get().
					Resource("nodes").
					FieldsSelectorParam(fields.Everything()).
					Do().
					Get()
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return lbController.client.Get().
					Prefix("watch").
					Resource("nodes").
					FieldsSelectorParam(fields.Everything()).
					Param("resourceVersion", options.ResourceVersion).Watch()
			},
		},
		&api.Node{}, 0, nodeHandlers)

	return &lbController, nil
}

// Run starts the configmap controller
func (lbController *LoadBalancerController) Run() {
	go lbController.nodeController.Run(lbController.stopCh)

	// Sleep for 3 seconds to give some times for service and node lister to be synced
	time.Sleep(time.Second * 3)
	go lbController.configMapController.Run(lbController.stopCh)
	go lbController.configMapQueue.run(time.Second, lbController.stopCh)
	<-lbController.stopCh
}

func configMapListFunc(c *client.Client, ns string, labelKey, labelValue string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		opts.LabelSelector = labels.Set{labelKey: labelValue}.AsSelector()
		return c.ConfigMaps(ns).List(opts)
	}
}

func configMapWatchFunc(c *client.Client, ns string, labelKey, labelValue string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		options.LabelSelector = labels.Set{labelKey: labelValue}.AsSelector()
		return c.ConfigMaps(ns).Watch(options)
	}
}

func (lbController *LoadBalancerController) syncConfigMap(key string) {
	glog.Infof("Syncing configmap %v", key)

	obj, _, err := lbController.configMapLister.Store.GetByKey(key)
	if err != nil {
		lbController.configMapQueue.requeue(key, err)
		return
	}
	// defaut/some-configmap -> default-some-configmap
	name := strings.Replace(key, "/", "-", -1)
	go func() {
		configMap := obj.(*api.ConfigMap)
		err := lbController.backendController.HandleConfigMapCreate(configMap)
		if err != nil {
			glog.Errorf("Error creating loadbalancer: %v", err)
			lbController.updateConfigMapStatusBindIP(err.Error(), "", configMap)
			return
		}
		bindIP, err := lbController.backendController.GetBindIP(name)
		if err != nil {
			err = fmt.Errorf("Error getting bind IP for %v configmap: %v", name, err)
			lbController.updateConfigMapStatusBindIP(err.Error(), "", configMap)
		} else if bindIP == "" {
			err = fmt.Errorf("No BindIP found for %v configmap", name)
			lbController.updateConfigMapStatusBindIP(err.Error(), "", configMap)
		} else {
			lbController.updateConfigMapStatusBindIP("", bindIP, configMap)
		}
	}()
}

func configmapsEqual(m1 map[string]string, m2 map[string]string) bool {
	return m1["namespace"] == m2["namespace"] && m1["target-service-name"] == m2["target-service-name"]
}

// update user configmap with status
func (lbController *LoadBalancerController) updateConfigMapStatusBindIP(errMessage string, bindIP string, configMap *api.ConfigMap) {
	configMapData := configMap.Data

	//set status
	if errMessage != "" {
		configMapData["status"] = "ERROR : " + errMessage
		delete(configMapData, "bind-ip")
	} else if bindIP != "" {
		configMapData["status"] = "SUCCESS"
		configMapData["bind-ip"] = bindIP
	}

	_, err := lbController.client.ConfigMaps(configMap.Namespace).Update(configMap)
	if err != nil {
		glog.Errorf("Error updating ConfigMap Status : %v", err)
	}
}
