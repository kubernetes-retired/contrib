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

	"github.com/chiradeep/contrib/ingress/controllers/citrix-netscaler/netscaler"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
)

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

func ingressToPolicyNames(ingress extensions.Ingress) []string {
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

func ingressToNetscalerConfig(kubeClient *client.Client, csvserverName string, ingress extensions.Ingress, priority int,
	knownEndpoints map[string]map[string]string) int {

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		namespace := ingress.Namespace
		thisIngEndpoints := make(map[string]string)
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
				netscaler.ConfigureContentVServer(namespace, csvserverName, host, path_, serviceIp, serviceName_mod, servicePort, priority)
			}
			priority += 10
		}
		ingName := ingress.Name + "_" + host + "_" + namespace
		knownEndpoints[ingName] = thisIngEndpoints
	}
	return priority
}

func createContentVserverForIngress(ing extensions.Ingress) (string, error) {
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
func endpointsCheck(kubeClient *client.Client, ingList *extensions.IngressList,
	knownEndpoints map[string]map[string]string) map[string]map[string]string {
	newEndpoints := make(map[string]map[string]string)

	for _, ingress := range ingList.Items {
		thisIngEndpoints := make(map[string]string)
		for _, rule := range ingress.Spec.Rules {
			host := rule.Host
			namespace := ingress.Namespace
			for _, path := range rule.HTTP.Paths {
				serviceName := path.Backend.ServiceName
				//servicePort := path.Backend.ServicePort.IntValue()

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
					if err != nil {
						log.Printf("Failed to convert endpoint port to integer %s", ep_ip_port[1])
						continue
					}
					serviceName_mod := "svc_" + serviceName + "_" + strings.Replace(serviceIp, ".", "_", -1) + "_" + ep_ip_port[1]
					thisIngEndpoints[ep] = serviceName_mod
				}
			}
			ingName := ingress.Name + "_" + host + "_" + namespace
			newEndpoints[ingName] = thisIngEndpoints

			knownThisIngEndpoints := knownEndpoints[ingName]
			if reflect.DeepEqual(knownThisIngEndpoints, thisIngEndpoints) {
				continue
			}

			// Find the items in known and not in new - Delete services
			for knownEpIP, sname := range knownThisIngEndpoints {
				_, prs := thisIngEndpoints[knownEpIP]
				if prs == false {
					//Delete the Netscaler Services
					netscaler.DeleteService(sname)
				}
			}

			// Find the items in new and not in known - Add services
			for newEpIP, sname := range thisIngEndpoints {
				_, prs := knownThisIngEndpoints[newEpIP]
				if prs == false {
					//Add Netscaler Service
					// Find lb name from ingress name to use when adding/binding service
					lbName := netscaler.GenerateLbName(namespace, host)
					netscaler.AddAndBindService(lbName, sname, newEpIP)
				}
			}

		}
	}

	//fmt.Println("  **** DBG newEndpoints map   : ", newEndpoints)
	//fmt.Println("  **** DBG knownEndpoints map : ", knownEndpoints)

	return newEndpoints
}

func loop(kubeClient *client.Client) {
	var ingClient client.IngressInterface
	ingClient = kubeClient.Extensions().Ingress(api.NamespaceAll)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}
	knownEndpoints := make(map[string]map[string]string)

	// Controller loop
	for {
		rateLimiter.Accept()
		ingresses, err := ingClient.List(api.ListOptions{})

		if err != nil {
			log.Printf("Error retrieving ingresses: %v", err)
			continue
		}

		if reflect.DeepEqual(ingresses.Items, known.Items) {
			// Ingress and known items are equal. Check if endpoints have changed.
			// Returns list of new known endpoints
			knownEndpoints = endpointsCheck(kubeClient, ingresses, knownEndpoints)
			continue
		}

		known = ingresses
		var existingCsVservers = sets.NewString()
		existingCsVservers.Insert(netscaler.ListContentVservers()...)

		for _, ing := range ingresses.Items {
			var newOrExistingPolicyNames = sets.NewString()
			var priority = 10

			log.Printf("inner loop: current priority: %d", priority)
			csvserverName, err := createContentVserverForIngress(ing)
			if err != nil {
				log.Printf("Unable to create / retrieve content vserver for ingress %s; skipping", ing.Name)
				continue
			}

			_, priorities := netscaler.ListBoundPolicies(csvserverName)
			if len(priorities) > 0 {
				priority = priorities[len(priorities)-1] + 10
			}
			priority = ingressToNetscalerConfig(kubeClient, csvserverName, ing, priority, knownEndpoints)

			policyNames := ingressToPolicyNames(ing)
			newOrExistingPolicyNames.Insert(policyNames...)
			log.Printf("New or Existing Ingress: %v", newOrExistingPolicyNames.List())

			var lbPolicyNames = sets.NewString()
			policyNames, _ = netscaler.ListBoundPolicies(csvserverName)
			lbPolicyNames.Insert(policyNames...)
			log.Printf("Existing Ingress policy on LB: %v", lbPolicyNames.List())

			toDelete := lbPolicyNames.Difference(newOrExistingPolicyNames)
			log.Printf("Need to delete: %v", toDelete.List())
			netscaler.DeleteCsPolicies(csvserverName, toDelete.List())
			existingCsVservers.Delete(csvserverName)
		}

		for _, csvserver := range existingCsVservers.List() {
			netscaler.DeleteContentVServer(csvserver)
		}

		// Check if endpoints have changed.
		knownEndpoints = endpointsCheck(kubeClient, ingresses, knownEndpoints)
	}
}

func main() {
	kube_apiserver_addr := os.Getenv("KUBERNETES_APISERVER_ADDR")
	kube_apiserver_port := os.Getenv("KUBERNETES_APISERVER_PORT")
	kube_apiserver_host := fmt.Sprintf("http://%s:%s", kube_apiserver_addr, kube_apiserver_port)
	config := restclient.Config{
		//Host:     "https://127.0.0.1:6443",
		//Host:     "http://10.217.129.67:8080",
		Host:     kube_apiserver_host,
		Insecure: true,
	}
	kubeClient, err := client.New(&config)
	if err != nil {
		log.Fatalln("Can't connect to Kubernetes API:", err)
	}
	loop(kubeClient)
}
