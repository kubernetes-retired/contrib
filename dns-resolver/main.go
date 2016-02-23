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
	// "io/ioutil"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	resolverConfig = "/etc/resolv.conf"
	healthzPort    = 8081
)

var (
	flags = pflag.NewFlagSet("", pflag.ContinueOnError)

	cluster = flags.Bool("use-kubernetes-cluster-service", true, `If true, use the
		built in kubernetes cluster for creating the client`)

	domain = flags.String("domain", "cluster.local", "domain under which to create names")

	clusterDNS = flags.String("cluster-dns", "", "IP address for a cluster DNS server")

	backendName = flags.String("backend", "nsd", "DNS server to use as backend")

	customForwards = flags.String("custom-forwards", "", `Custom domain forwards separated by comma. 
		Each domain must indicate the IP address to wich the queries must be forwarded. If the dns port
		is not 53 is possible to change the default using @ as separator between the IP address and the port.
		Example: www.k8s.io:1.1.1.1,www.kubernetes.io:1.1.1.2@54`)
)

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	clientConfig := kubectl_util.DefaultClientConfig(flags)

	var err error
	var kubeClient *unversioned.Client

	if *cluster {
		if kubeClient, err = unversioned.NewInCluster(); err != nil {
			glog.Fatalf("Failed to create client: %v", err)
		}
	} else {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = unversioned.New(config)
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
	}

	if *clusterDNS == "" {
		glog.Fatalf("cluster-dns flag not specified")
	}

	ks := newController(*domain, *clusterDNS)

	switch *backendName {
	case "nsd":
		ks.backend = newNsdBackend(*domain, *clusterDNS)
	default:
		glog.Fatalf("invalid backend name (%v)", backendName)
	}

	ks.endpointsStore = watchEndpoints(kubeClient, ks)
	ks.servicesStore = watchForServices(kubeClient, ks)
	ks.podsStore = watchPods(kubeClient, ks)

	dnsResolver := &resolver{
		domain:  *domain,
		ns:      parseResolvConf(),
		forward: parseForwards(*customForwards),
	}
	ks.resolver = dnsResolver

	go ks.backend.Start()
	go registerHandlers(ks, dnsResolver)

	dnsResolver.Start()
}

func registerHandlers(k2dns *kube2dns, cache *resolver) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if cache.IsHealthy() && k2dns.backend.IsHealthy() {
			w.WriteHeader(200)
			w.Write([]byte("ok"))
			return
		}

		w.WriteHeader(500)
		w.Write([]byte("unhealthy"))
	})

	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", healthzPort), nil))
}

func parseResolvConf() []string {
	// default server to use in case of errors reading /etc/resolv.conf
	// TODO: return the default only if
	return []string{"8.8.8.8"}

	// f, err := os.Open(resolverConfig)
	// if err != nil {
	// 	return defSearch
	// }

	// file, err := ioutil.ReadAll(f)
	// if err != nil {
	// 	return defSearch
	// }

	// nameservers := []string{}
	// lines := strings.Split(string(file), "\n")
	// for l := range lines {
	// 	trimmed := strings.TrimSpace(lines[l])
	// 	if strings.HasPrefix(trimmed, "#") {
	// 		continue
	// 	}
	// 	fields := strings.Fields(trimmed)
	// 	if len(fields) == 0 {
	// 		continue
	// 	}
	// 	if fields[0] == "nameserver" {
	// 		nameservers = append(nameservers, fields[1:]...)
	// 	}
	// }

	// return nameservers
}

func parseForwards(input string) []forward {
	forwards := []forward{}
	domains := strings.Split(input, ",")
	for _, domain := range domains {
		domainPort := strings.Split(domain, ":")
		if len(domainPort) == 2 {
			forwards = append(forwards, forward{
				Name: domainPort[0],
				IP:   domainPort[1],
			})
		} else {
			glog.V(2).Infof("invalid forward format (%v)", domainPort)
		}
	}
	return forwards
}
