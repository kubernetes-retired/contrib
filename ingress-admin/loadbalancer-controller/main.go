/*
Copyright 2016 The Kubernetes Authors.

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
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/client-go/1.5/dynamic"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/errors"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/util/intstr"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/rest"

	"k8s.io/contrib/ingress-admin/loadbalancer-controller/controller"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/loadbalancerprovider"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/loadbalancerprovider/providers"
)

var (
	flags = pflag.NewFlagSet("", pflag.ExitOnError)
)

func init() {
	flag.Set("logtostderr", "true")
	flag.Parse()
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
}

func init() {
	loadbalancerprovider.RegisterPlugin(nginx.ProbeLoadBalancerPlugin())
}

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	// workaround of noisy log, see https://github.com/kubernetes/kubernetes/issues/17162
	flag.CommandLine.Parse([]string{})

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	if err := ensureDefaulBackendService(clientset); err != nil {
		panic(err)
	}

	// create dynamic client
	resources := []*unversioned.APIResourceList{
		{
			GroupVersion: "k8s.io/v1",
			APIResources: []unversioned.APIResource{
				{
					Name:       "loadbalancerclaims",
					Namespaced: true,
					Kind:       "loadbalancerclaim",
				},
				{
					Name:       "loadbalancers",
					Namespaced: true,
					Kind:       "loadbalancer",
				},
			},
		},
	}
	mapper, err := dynamic.NewDiscoveryRESTMapper(resources, dynamic.VersionInterfaces)
	if err != nil {
		panic(err.Error())
	}
	dynamicClient, err := dynamic.NewClientPool(config, mapper, dynamic.LegacyAPIPathResolverFunc).
		ClientForGroupVersionKind(unversioned.GroupVersionKind{Group: "k8s.io", Version: "v1"})
	if err != nil {
		panic(err.Error())
	}

	pc := controller.NewProvisionController(clientset, dynamicClient, loadbalancerprovider.PluginMgr)
	pc.Run(5, wait.NeverStop)

}

func ensureDefaulBackendService(clientset *kubernetes.Clientset) error {
	svc := v1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "default-http-backend",
			Labels: map[string]string{
				"app": "default-http-backend",
			},
		},
		Spec: v1.ServiceSpec{
			Type:            v1.ServiceTypeClusterIP,
			SessionAffinity: v1.ServiceAffinityNone,
			Selector: map[string]string{
				"app": "default-http-backend",
			},
			Ports: []v1.ServicePort{
				{
					Port:       int32(80),
					TargetPort: intstr.FromInt(8080),
					Protocol:   v1.ProtocolTCP,
				},
			},
		},
	}

	if _, err := clientset.Core().Services("default").Create(&svc); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
