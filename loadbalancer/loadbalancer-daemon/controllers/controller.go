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
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/glog"

	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer-daemon/keepalived"
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// ConfigMapController watches Kubernetes API for ConfigMap changes
// and reconfigures keepalived and backend when needed
type ConfigMapController struct {
	client              *client.Client
	configMapController *framework.Controller
	configMapLister     StoreToConfigMapLister
	stopCh              chan struct{}
	backendController   factory.BackendController
	keepaliveController keepalived.KeepalivedController
}

// StoreToConfigMapLister makes a Store that lists ConfigMap.
type StoreToConfigMapLister struct {
	cache.Store
}

// Values to verify the configmap object is a loadbalancer config
const (
	configLabelKey   = "loadbalancer"
	configLabelValue = "daemon"
)

var keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc

// NewConfigMapController creates a controller
func NewConfigMapController(kubeClient *client.Client, resyncPeriod time.Duration, namespace string, controller factory.BackendController, runKeepalived bool) (*ConfigMapController, error) {

	// Initializing configmap and backend controller
	configMapController := ConfigMapController{
		client:            kubeClient,
		stopCh:            make(chan struct{}),
		backendController: controller,
	}

	if runKeepalived {
		// Initializing keepalived controller
		nodeInterface := getNodeInterface(kubeClient)
		configMapController.keepaliveController = keepalived.NewKeepalivedController(nodeInterface)
		configMapController.keepaliveController.Start()

		// Listen for keepalived process monitor signal
		go func() {
			<-configMapController.keepaliveController.ExitChannel()
			glog.Infof("Keepalived process has stopped. Cleaning VIPs")
			configMapController.keepaliveController.Clean()
			// Quit the application
			os.Exit(255)
		}()
	}

	// Listen for nginx process monitor signal
	go func() {
		<-configMapController.backendController.ExitChannel()
		glog.Infof("Nginx process has stopped.")
		if runKeepalived {
			configMapController.keepaliveController.Clean()
		}
		// Quit the application
		os.Exit(255)
	}()

	configMapHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm := obj.(*api.ConfigMap)
			cmData := cm.Data

			groups := utils.GetConfigMapGroups(cmData)
			for group := range groups {

				if runKeepalived {
					go configMapController.keepaliveController.AddVIP(cmData[group+".bind-ip"])
				}

				backendConfig := createBackendConfig(cmData, group)
				go configMapController.backendController.AddConfig(group, backendConfig)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if runKeepalived {
				go configMapController.keepaliveController.DeleteAllVIPs()
			}
			cm := obj.(*api.ConfigMap)
			cmData := cm.Data
			groups := utils.GetConfigMapGroups(cmData)
			for group := range groups {
				go configMapController.backendController.DeleteConfig(group)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				oldCM := old.(*api.ConfigMap).Data
				curCM := cur.(*api.ConfigMap).Data
				groups := utils.GetConfigMapGroups(curCM)
				updatedGroups := utils.GetUpdatedConfigMapGroups(oldCM, curCM)
				for group := range updatedGroups {
					if !groups.Has(group) {
						if runKeepalived {
							go configMapController.keepaliveController.DeleteVIP(oldCM[group+".bind-ip"])
						}
						go configMapController.backendController.DeleteConfig(group)
					} else {
						if runKeepalived {
							go configMapController.keepaliveController.AddVIP(curCM[group+".bind-ip"])
						}
						backendConfig := createBackendConfig(curCM, group)
						go configMapController.backendController.AddConfig(group, backendConfig)
					}
				}
			}
		},
	}
	configMapController.configMapLister.Store, configMapController.configMapController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  configMapListFunc(kubeClient, namespace),
			WatchFunc: configMapWatchFunc(kubeClient, namespace),
		},
		&api.ConfigMap{}, resyncPeriod, configMapHandlers)

	return &configMapController, nil
}

// Run starts the configmap controller
func (configMapController *ConfigMapController) Run() {
	go configMapController.configMapController.Run(configMapController.stopCh)
	<-configMapController.stopCh
}

func configMapListFunc(c *client.Client, ns string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		opts.LabelSelector = labels.Set{configLabelKey: configLabelValue}.AsSelector()
		return c.ConfigMaps(ns).List(opts)
	}
}

func configMapWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		options.LabelSelector = labels.Set{configLabelKey: configLabelValue}.AsSelector()
		return c.ConfigMaps(ns).Watch(options)
	}
}

func createBackendConfig(cm map[string]string, group string) factory.BackendConfig {
	// Get all the ports
	i := 0
	ports := []string{}
	for port, exist := cm[group+".port"+strconv.Itoa(i)]; exist; {
		ports = append(ports, port)
		i++
		port, exist = cm[group+".port"+strconv.Itoa(i)]
	}

	ssl, _ := strconv.ParseBool(cm[group+".SSL"])
	sslPort, _ := strconv.Atoi(cm[group+".ssl-port"])
	backendConfig := factory.BackendConfig{
		Host:              cm[group+".host"],
		Namespace:         cm[group+".namespace"],
		BindIp:            cm[group+".bind-ip"],
		Ports:             ports,
		TargetServiceName: cm[group+".target-service-name"],
		TargetIP:          cm[group+".target-ip"],
		SSL:               ssl,
		SSLPort:           sslPort,
		Path:              cm[group+".path"],
		TlsCert:           "some cert", //TODO get certs from secret
		TlsKey:            "some key",  //TODO get certs from secret
	}
	return backendConfig
}

// Get node interface for the pod node IP
func getNodeInterface(kubeClient *client.Client) string {

	// Get node IP from the pod
	podName := os.Getenv("POD_NAME")
	podNs := os.Getenv("POD_NAMESPACE")

	if podName == "" || podNs == "" {
		glog.Fatalf("Please check the manifest (for missing POD_NAME or POD_NAMESPACE env variables)")
	}

	pod, _ := kubeClient.Pods(podNs).Get(podName)
	if pod == nil {
		glog.Fatalf("Unable to get POD information")
	}

	node, err := kubeClient.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		glog.Fatalf("Unable to get NODE with name %s", pod.Spec.NodeName)
	}

	nodeIP, err := utils.GetNodeHostIP(*node)
	if err != nil {
		glog.Fatalf("Error while getting IP for %s. %v", node.Name, err)
	}
	iface := interfaceByIP(*nodeIP)
	if iface == "" {
		glog.Fatalf("Cannot find interface for IP: %v", nodeIP)
	}
	return iface
}
