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
	"log"
	"reflect"
	"strconv"

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

func ingressToNetscalerConfig(kubeClient *client.Client, csvserverName string, ingress extensions.Ingress, priority int) int {

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		namespace := ingress.Namespace
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
			// Need to resolve the service IP
			s, err := kubeClient.Services(api.NamespaceDefault).Get(serviceName)
			if err != nil {
				log.Printf("Failed to retrieve Service %s", serviceName)
				continue
			}
			serviceIp := s.Spec.ClusterIP
			if serviceIp == "None" {
				log.Printf("Service %s has service IP of None", serviceName)
			}
			log.Printf("Configure Netscaler: policy: %s Ingress Host: %s, path: %s, serviceName: %s, serviceIp: %s servicePort: %d priority %d", policyName, host, path_, serviceName, serviceIp, servicePort, priority)
			netscaler.ConfigureContentVServer(namespace, csvserverName, host, path_, serviceIp, serviceName, servicePort, priority)
			priority += 10
		}
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

func loop(kubeClient *client.Client) {
	var ingClient client.IngressInterface
	ingClient = kubeClient.Extensions().Ingress(api.NamespaceAll)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}

	// Controller loop
	for {
		rateLimiter.Accept()
		ingresses, err := ingClient.List(api.ListOptions{})
		if err != nil {
			log.Printf("Error retrieving ingresses: %v", err)
			continue
		}
		if reflect.DeepEqual(ingresses.Items, known.Items) {
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
			priority = ingressToNetscalerConfig(kubeClient, csvserverName, ing, priority)
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

	}
}

func main() {
	config := restclient.Config{
		Host:     "https://127.0.0.1:6443",
		Insecure: true,
	}
	kubeClient, err := client.New(&config)
	if err != nil {
		log.Fatalln("Can't connect to Kubernetes API:", err)
	}
	loop(kubeClient)
}
