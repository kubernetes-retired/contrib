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
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
	"netscaler"
)

type StoreToIngressLister struct {
	cache.Store
}

var priority = 10
var knownEndpoints = make(map[string]map[string]string)
var svcname_refcount = make(map[string]int)                // Reference count of NS full service name
var ing_svcname_refcount = make(map[string]map[string]int) // Reference count of ingresses per kubernetes service
var lbNameMap = make(map[string]int)

func ingressRuleToPolicyName(namespace string, rule extensions.IngressRule) []string {
	resultPolicyNames := []string{}
	host := rule.Host
	for _, path := range rule.HTTP.Paths {
		path_ := path.Path
		serviceName := path.Backend.ServiceName
		servicePort := path.Backend.ServicePort.IntValue()
		log.Printf("Ingress: Host: %s, path: %s, serviceName: %s, servicePort: %d", host, path_, serviceName, servicePort)
		policyName := netscaler.GeneratePolicyName(namespace, host, path_)
		resultPolicyNames = append(resultPolicyNames, policyName)
	}
	return resultPolicyNames
}

func ingressToPolicyNames(ingress *extensions.Ingress) []string {
	resultPolicyNames := []string{}
	namespace := ingress.Namespace
	for _, rule := range ingress.Spec.Rules {
		policyNames := ingressRuleToPolicyName(namespace, rule)
		resultPolicyNames = append(resultPolicyNames, policyNames...)
	}
	return resultPolicyNames
}

// Pass ports=nil for all ports.
func formatEndpoints(endpoints *api.Endpoints, ports sets.String) string {
	if len(endpoints.Subsets) == 0 {
		return "<none>"
	}
	list := []string{}
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		for i := range ss.Ports {
			port := &ss.Ports[i]
			if ports == nil || ports.Has(port.Name) {
				for i := range ss.Addresses {
					addr := &ss.Addresses[i]
					list = append(list, fmt.Sprintf("%s:%d", addr.IP, port.Port))
				}
			}
		}
	}
	ret := strings.Join(list, ",")
	return ret
}

func ingressToNetscalerConfig(kubeClient *client.Client, csvserverName string, ingress *extensions.Ingress, priority int,
	knownEndpoints map[string]map[string]string, svcname_refcount map[string]int,
	ing_svcname_refcount map[string]map[string]int) int {

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		namespace := ingress.Namespace
		thisIngEndpoints := make(map[string]string)
		var lbName string
		for _, path := range rule.HTTP.Paths {
			path_ := path.Path
			serviceName := path.Backend.ServiceName
			servicePort := path.Backend.ServicePort.IntValue()
			policyName := netscaler.GeneratePolicyName(namespace, host, path_)
			existing := netscaler.ListBoundPolicy(csvserverName, policyName)
			if len(existing) != 0 {
				log.Printf("Ingress: Policy already exists: %s, Host: %s, path: %s, serviceName: %s, servicePort: %d", policyName, host, path_, serviceName, servicePort)
				continue
			}

			// Find endpoints
			endpoints, err := kubeClient.Endpoints(api.NamespaceDefault).Get(serviceName)
			if err != nil {
				log.Printf("Failed to retrieve endpoints for service %s", serviceName)
				continue
			}
			endpoints_all := formatEndpoints(endpoints, nil)

			endpoints_split := strings.Split(endpoints_all, ",")
			for _, ep := range endpoints_split {
				ep_ip_port := strings.Split(ep, ":")
				serviceIp := ep_ip_port[0]
				servicePort, err := strconv.Atoi(ep_ip_port[1])
				if err != nil {
					log.Printf("Failed to convert endpoint port to integer %s", ep_ip_port[1])
					continue
				}

				serviceName_mod := "svc_" + serviceName + "_" + strings.Replace(serviceIp, ".", "_", -1) + "_" + ep_ip_port[1]
				thisIngEndpoints[ep] = serviceName_mod

				log.Printf("Configure Netscaler: policy: %s Ingress Host: %s, path: %s, serviceName: %s, serviceIp: %s servicePort: %d priority %d", policyName, host, path_, serviceName, serviceIp, servicePort, priority)
				lbName = netscaler.ConfigureContentVServer(namespace, csvserverName, host, path_, serviceIp, serviceName_mod, servicePort, priority, svcname_refcount)
				lbNameMap[lbName] = 1
			}
			priority += 10
			knownEndpoints[serviceName] = thisIngEndpoints
			ing_svcname_refcount[serviceName] = lbNameMap
		}
	}

	//fmt.Println("DBG knownEndpoints map : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
	return priority
}

func createContentVserverForIngress(ing *extensions.Ingress) (string, error) {
	csvserverName := netscaler.GenerateCsVserverName(ing.Namespace, ing.Name)
	if netscaler.FindContentVserver(csvserverName) {
		return csvserverName, nil
	}
	protocol, ok := ing.Annotations["protocol"]
	if !ok {
		protocol = "HTTP"
	}
	port, ok := ing.Annotations["port"]
	if !ok {
		port = "80"
	}
	intPort, err := strconv.Atoi(port)
	if err != nil {
		log.Printf("Failed to parse port annotation for ingress %s", ing.Name)
		return "", errors.New("Failed to parse port annotation for ingress " + ing.Name)
	}
	publicIP, ok := ing.Annotations["publicIP"]
	if !ok {
		log.Printf("Failed to retrieve annotation publicIP for ingress %s, skipping processing", ing.Name)
		return "", errors.New("Failed to retrieve annotation publicIP for ingress " + ing.Name)
	}
	err = netscaler.CreateContentVServer(csvserverName, publicIP, intPort, protocol)
	if err != nil {
		return "", errors.New("Failed to create content vserver " + csvserverName + " for ingress " + ing.Name)
	}
	return csvserverName, nil
}

/* Function to see if the endpoints have changed for a given ingress. If so the
 * NS serivices associated with them should be added or removed depending on
 * whether the endpoint is newly seen or a previous endpoint is no longer
 * avaialble.
 */
func updateEndpoints(knownEndpoints map[string]string,
	thisIngKnownEndpoints map[string]string,
	ingServiceName string,
	svcname_refcount map[string]int) {
	// Find the items in known and not in new - Delete services
	for knownEpIP, sname := range knownEndpoints {
		_, prs := thisIngKnownEndpoints[knownEpIP]
		if prs == false {
			//Delete the Netscaler Services
			netscaler.DeleteService(sname)
			serviceName_mod := "svc_" + ingServiceName + "_" + strings.Replace(knownEpIP, ".", "_", -1)
			serviceName_mod = strings.Replace(serviceName_mod, ":", "_", -1)
			_, present := svcname_refcount[serviceName_mod]
			if present {
				delete(svcname_refcount, serviceName_mod)
			}
		}
	}

	// Find the items in new and not in known - Add services
	for newEpIP, sname := range thisIngKnownEndpoints {
		_, prs := knownEndpoints[newEpIP]
		if prs == false {
			//Add Netscaler Service
			lbNames_map := ing_svcname_refcount[ingServiceName]
			for lbName, _ := range lbNames_map {
				netscaler.AddAndBindService(lbName, sname, newEpIP)
				serviceName_mod := "svc_" + ingServiceName + "_" + strings.Replace(newEpIP, ".", "_", -1)
				serviceName_mod = strings.Replace(serviceName_mod, ":", "_", -1)
				_, present := svcname_refcount[serviceName_mod]
				if present {
					svcname_refcount[serviceName_mod]++
				} else {
					svcname_refcount[serviceName_mod] = 1
				}
			}
		}
	}
}

func addIngress(kubeClient *client.Client, ing *extensions.Ingress) {
	log.Printf("inner loop: current priority: %d", priority)
	csvserverName, err := createContentVserverForIngress(ing)
	if err != nil {
		log.Printf("Unable to create / retrieve content vserver for ingress %s; skipping", ing.Name)
		return
	}

	_, priorities := netscaler.ListBoundPolicies(csvserverName)
	if len(priorities) > 0 {
		priority = priorities[len(priorities)-1] + 10
	}
	priority = ingressToNetscalerConfig(kubeClient, csvserverName, ing, priority, knownEndpoints, svcname_refcount, ing_svcname_refcount)
	//fmt.Println("DBG svcref map ADD  : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
}

func delIngress(kubeClient *client.Client, ing *extensions.Ingress) {
	csvserverName := netscaler.GenerateCsVserverName(ing.Namespace, ing.Name)
	for _, rule := range ing.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			serviceName := path.Backend.ServiceName
			netscaler.DeleteContentVServer(csvserverName, svcname_refcount, ing_svcname_refcount[serviceName])
			lbName_map := ing_svcname_refcount[serviceName]
			lbNameMap = lbName_map
			if len(lbName_map) == 0 {
				delete(ing_svcname_refcount, serviceName)
				delete(knownEndpoints, serviceName)
			}
		}
	}
	//fmt.Println("DBG svcref map DEL  : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
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

func epListFunc(c *client.Client, ns string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return c.Endpoints(ns).List(opts)
	}
}

func epWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return c.Endpoints(ns).Watch(options)
	}
}

func startControllers(kubeClient *client.Client) {
	var ingController *framework.Controller
	var epController *framework.Controller
	var ingLister StoreToIngressLister
	var epLister cache.StoreToEndpointsLister
	resyncPeriod := 10 * time.Second

	ingHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*extensions.Ingress)
			addIngress(kubeClient, addIng)
		},
		DeleteFunc: func(obj interface{}) {
			delIng := obj.(*extensions.Ingress)
			delIngress(kubeClient, delIng)
		},
	}

	epHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addEP := obj.(*api.Endpoints)
			endpoints_all := formatEndpoints(addEP, nil)
			_, found := ing_svcname_refcount[addEP.Name]
			if found {
				thisIngEndpoints := make(map[string]string)
				endpoints_split := strings.Split(endpoints_all, ",")
				for _, ep := range endpoints_split {
					ep_ip_port := strings.Split(ep, ":")
					serviceIp := ep_ip_port[0]
					serviceName_mod := "svc_" + addEP.Name + "_" + strings.Replace(serviceIp, ".", "_", -1) + "_" + ep_ip_port[1]
					thisIngEndpoints[ep] = serviceName_mod
				}
				knownEndpoints[addEP.Name] = thisIngEndpoints
			}
			//fmt.Println("DBG knownEndpoints map : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
		},
		DeleteFunc: func(obj interface{}) {
			delEP := obj.(*api.Endpoints)
			endpoints_all := formatEndpoints(delEP, nil)
			_, found := ing_svcname_refcount[delEP.Name]
			if found {
				if endpoints_all == "<none>" {
					delete(knownEndpoints, delEP.Name)
				} else {
					thisIngEndpoints := make(map[string]string)
					endpoints_split := strings.Split(endpoints_all, ",")
					for _, ep := range endpoints_split {
						ep_ip_port := strings.Split(ep, ":")
						serviceIp := ep_ip_port[0]
						serviceName_mod := "svc_" + delEP.Name + "_" + strings.Replace(serviceIp, ".", "_", -1) + "_" + ep_ip_port[1]
						thisIngEndpoints[ep] = serviceName_mod
					}
					knownEndpoints[delEP.Name] = thisIngEndpoints
				}
			}
			//fmt.Println("DBG knownEndpoints map : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				upEP := cur.(*api.Endpoints)
				endpoints_all := formatEndpoints(upEP, nil)
				_, found := ing_svcname_refcount[upEP.Name]
				if found {
					thisIngEndpoints := make(map[string]string)
					if endpoints_all != "<none>" {
						endpoints_split := strings.Split(endpoints_all, ",")
						for _, ep := range endpoints_split {
							ep_ip_port := strings.Split(ep, ":")
							serviceIp := ep_ip_port[0]
							serviceName_mod := "svc_" + upEP.Name + "_" + strings.Replace(serviceIp, ".", "_", -1) + "_" + ep_ip_port[1]
							thisIngEndpoints[ep] = serviceName_mod
						}
					}
					updateEndpoints(knownEndpoints[upEP.Name], thisIngEndpoints, upEP.Name, svcname_refcount)
					knownEndpoints[upEP.Name] = thisIngEndpoints
				}
				//fmt.Println("DBG knownEndpoints map : ", knownEndpoints, svcname_refcount, ing_svcname_refcount, lbNameMap)
			}
		},
	}

	ingLister.Store, ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(kubeClient, api.NamespaceAll),
			WatchFunc: ingressWatchFunc(kubeClient, api.NamespaceAll),
		},
		&extensions.Ingress{}, resyncPeriod, ingHandlers)

	epLister.Store, epController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  epListFunc(kubeClient, api.NamespaceAll),
			WatchFunc: epWatchFunc(kubeClient, api.NamespaceAll),
		},
		&api.Endpoints{}, resyncPeriod, epHandlers)

	stop := make(chan struct{})
	go ingController.Run(stop)
	go epController.Run(stop)
	<-stop
	log.Printf("ABK Exiting")
}

func main() {
	// Prefer environment variables and otherwise try accessing APIserver directly
	var kubeClient *client.Client
	var err error
	kube_apiserver_addr := os.Getenv("KUBERNETES_APISERVER_ADDR")
	kube_apiserver_port := os.Getenv("KUBERNETES_APISERVER_PORT")
	if (kube_apiserver_addr != "") && (kube_apiserver_port != "") {
		kube_apiserver_host := fmt.Sprintf("http://%s:%s", kube_apiserver_addr, kube_apiserver_port)
		config := restclient.Config{
			Host:     kube_apiserver_host,
			Insecure: true,
		}
		kubeClient, err = client.New(&config)
	} else {
		kubeClient, err = client.NewInCluster()
	}
	if err != nil {
		log.Fatalln("Can't connect to Kubernetes API:", err)
	}

	// Performing cleanup - start with a clean NS config. Handle situations where
	// k8s cluster has changed while NS has stale configuration.
	var existingCsVservers = sets.NewString()
	existingCsVservers.Insert(netscaler.ListContentVservers()...)
	for _, csvserver := range existingCsVservers.List() {
		netscaler.DeleteContentVServer(csvserver, svcname_refcount, nil)
	}

	startControllers(kubeClient)
}
