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
	"log"
	"os"
	"reflect"
	"sort"

	"github.com/chiradeep/contrib/ingress/controllers/citrix-netscaler/netscaler"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

func ingressToNetscalerConfig(kubeClient *client.Client, csvserverName string, ingress extensions.Ingress, priority int) []string {
	var resultPolicyNames []string
	namespace := ingress.Namespace
	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		for _, path := range rule.HTTP.Paths {
			path_ := path.Path
			serviceName := path.Backend.ServiceName
			servicePort := path.Backend.ServicePort.IntValue()
			log.Printf("Host: %s, path: %s, serviceName: %s, servicePort: %d", host, path_, serviceName, servicePort)
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
			log.Printf("Host: %s, path: %s, serviceName: %s, serviceIp: %s servicePort: %d priority %d", host, path_, serviceName, serviceIp, servicePort, priority)
			netscaler.ConfigureContentVServer(namespace, csvserverName, host, path_, serviceIp, serviceName, servicePort, priority)
			policyName := netscaler.GeneratePolicyName(namespace, host, path_)
			resultPolicyNames = append(resultPolicyNames, policyName)
			priority += 10
		}

	}
	return resultPolicyNames
}

func sliceDifference(left []string, right []string) []string {
	sort.Strings(right)
	lenRight := len(right)
	var result []string
	for _, s := range left {
		index := sort.SearchStrings(right, s)
		log.Printf("s=%s, searchResult=%d", s, index)
		if index == lenRight || right[index] != s {
			result = append(result, s)
		}
	}
	return result
}

func loop(csvserverName string, kubeClient *client.Client) {
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
		var newOrExistingPolicyNames []string
		priority := 10
		for _, ing := range ingresses.Items {
			policyNames := ingressToNetscalerConfig(kubeClient, csvserverName, ing, priority)
			newOrExistingPolicyNames = append(newOrExistingPolicyNames, policyNames...)
		}
		lbPolicyNames := netscaler.ListBoundPolicies(csvserverName)
		toDelete := sliceDifference(lbPolicyNames, newOrExistingPolicyNames)
		netscaler.DeleteCsPolicies(csvserverName, toDelete)
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
	csvserver := os.Getenv("NS_CSVSERVER")
	_ = csvserver
	loop(csvserver, kubeClient)
}
