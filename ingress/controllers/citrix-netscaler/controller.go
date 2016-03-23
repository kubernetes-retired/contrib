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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

func main() {
	var ingClient client.IngressInterface
	config := client.Config{
		Host:     "https://127.0.0.1:6443",
		Insecure: true,
	}
	kubeClient, err := client.New(&config)
	if err != nil {
		log.Fatalln("Can't connect to Kubernetes API:", err)
	}
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
		/*
			{{range $ing := .Items}}
			{{range $rule := $ing.Spec.Rules}}
			  server {
			    listen 80;
			    server_name {{$rule.Host}};
			{{ range $path := $rule.HTTP.Paths }}
			    location {{$path.Path}} {
			      proxy_set_header Host $host;
			      proxy_pass http://{{$path.Backend.ServiceName}}.{{$ing.Namespace}}.svc.cluster.local:{{$path.Backend.ServicePort}};
			    }{{end}}
			  }{{end}}{{end}}
		*/
		for _, ing := range ingresses.Items {
			for _, rule := range ing.Spec.Rules {
				host := rule.Host
				for _, path := range rule.HTTP.Paths {
					path_ := path.Path
					serviceName := path.Backend.ServiceName
					servicePort := path.Backend.ServicePort
					log.Printf("Host: %s, path: %s, serviceName: %s, servicePort: %d", host, path_, serviceName, servicePort.IntValue())
				}

			}

		}
		os.Exit(1)
	}

}
