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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
)

// storeEps stores the given endpoints in a store.
func storeEps(eps []*api.Endpoints) cache.Store {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	found := make([]interface{}, 0, len(eps))
	for i := range eps {
		found = append(found, eps[i])
	}
	if err := store.Replace(found, "0"); err != nil {
		glog.Fatalf("Unable to replace endpoints %v", err)
	}
	return store
}

// storeServices stores the given services in a store.
func storeServices(svcs []*api.Service) cache.Store {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	found := make([]interface{}, 0, len(svcs))
	for i := range svcs {
		found = append(found, svcs[i])
	}
	if err := store.Replace(found, "0"); err != nil {
		glog.Fatalf("Unable to replace services %v", err)
	}
	return store
}

func getEndpoints(svc *api.Service, endpointAddresses []api.EndpointAddress, endpointPorts []api.EndpointPort) *api.Endpoints {
	return &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Subsets: []api.EndpointSubset{{
			Addresses: endpointAddresses,
			Ports:     endpointPorts,
		}},
	}
}

func getService(servicePorts []api.ServicePort) *api.Service {
	return &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name: string(util.NewUUID()), Namespace: api.NamespaceDefault},
		Spec: api.ServiceSpec{
			Ports: servicePorts,
		},
	}
}

func newFakeLoadBalancerController(endpoints []*api.Endpoints, services []*api.Service) *loadBalancerController {
	flb := loadBalancerController{}
	flb.epLister.Store = storeEps(endpoints)
	flb.svcLister.Store = storeServices(services)
	flb.httpPort = 80
	return &flb
}

func TestGetEndpoints(t *testing.T) {
	// 2 pods each of which have 3 targetPorts exposed via a single service
	endpointAddresses := []api.EndpointAddress{
		{IP: "1.2.3.4"},
		{IP: "6.7.8.9"},
	}
	ports := []int{80, 443, 3306}
	endpointPorts := []api.EndpointPort{
		{Port: ports[0], Protocol: "TCP"},
		{Port: ports[1], Protocol: "TCP"},
		{Port: ports[2], Protocol: "TCP", Name: "mysql"},
	}
	servicePorts := []api.ServicePort{
		{Port: ports[0], TargetPort: intstr.FromInt(ports[0])},
		{Port: ports[1], TargetPort: intstr.FromInt(ports[1])},
		{Port: ports[2], TargetPort: intstr.FromString("mysql")},
	}

	svc := getService(servicePorts)
	endpoints := []*api.Endpoints{getEndpoints(svc, endpointAddresses, endpointPorts)}
	flb := newFakeLoadBalancerController(endpoints, []*api.Service{svc})

	for i := range ports {
		eps := flb.getEndpoints(svc, &svc.Spec.Ports[i])
		expectedEps := sets.NewString()
		for _, address := range endpointAddresses {
			expectedEps.Insert(fmt.Sprintf("%v:%v", address.IP, ports[i]))
		}

		receivedEps := sets.NewString()
		for _, ep := range eps {
			receivedEps.Insert(ep)
		}
		if len(receivedEps) != len(expectedEps) || !expectedEps.IsSuperset(receivedEps) {
			t.Fatalf("Unexpected endpoints, received %+v, expected %+v", receivedEps, expectedEps)
		}
		glog.Infof("Got endpoints %+v", receivedEps)
	}
}

func TestGetServices(t *testing.T) {
	endpointAddresses := []api.EndpointAddress{
		{IP: "1.2.3.4"},
		{IP: "6.7.8.9"},
	}
	ports := []int{80, 443}
	endpointPorts := []api.EndpointPort{
		{Port: ports[0], Protocol: "TCP"},
		{Port: ports[1], Protocol: "TCP"},
	}
	servicePorts := []api.ServicePort{
		{Port: 10, TargetPort: intstr.FromInt(ports[0])},
		{Port: 20, TargetPort: intstr.FromInt(ports[1])},
	}

	// 2 services targeting the same endpoints, one of which is declared as a tcp service.
	svc1 := getService(servicePorts)
	svc2 := getService(servicePorts)
	endpoints := []*api.Endpoints{
		getEndpoints(svc1, endpointAddresses, endpointPorts),
		getEndpoints(svc2, endpointAddresses, endpointPorts),
	}

	flb := newFakeLoadBalancerController(endpoints, []*api.Service{svc1, svc2})
	cfg, _ := filepath.Abs("./test-samples/loadbalancer_test.json")
	flb.cfg = parseCfg(cfg, "roundrobin", "", "")
	flb.tcpServices = map[string]int{
		svc1.Name: 20,
	}
	http, _, tcp := flb.getServices()
	serviceURLEp := fmt.Sprintf("%v:%v", svc1.Name, 20)
	if len(tcp) != 1 || tcp[0].Name != serviceURLEp || tcp[0].FrontendPort != 20 {
		t.Fatalf("Unexpected tcp service %+v expected %+v", tcp, svc1.Name)
	}

	// All pods of svc1 exposed under servicePort 20 are tcp
	expectedTCPEps := sets.NewString()
	for _, address := range endpointAddresses {
		expectedTCPEps.Insert(fmt.Sprintf("%v:%v", address.IP, 443))
	}
	receivedTCPEps := sets.NewString()
	for _, ep := range tcp[0].Ep {
		receivedTCPEps.Insert(ep)
	}
	if len(receivedTCPEps) != len(expectedTCPEps) || !expectedTCPEps.IsSuperset(receivedTCPEps) {
		t.Fatalf("Unexpected tcp serice %+v", tcp)
	}

	// All pods of either service not mentioned in the tcpmap are multiplexed on port  :80 as http services.
	expectedURLMapping := map[string]sets.String{
		fmt.Sprintf("%v:%v", svc1.Name, 10): sets.NewString("1.2.3.4:80", "6.7.8.9:80"),
		fmt.Sprintf("%v:%v", svc2.Name, 10): sets.NewString("1.2.3.4:80", "6.7.8.9:80"),
		fmt.Sprintf("%v:%v", svc2.Name, 20): sets.NewString("1.2.3.4:443", "6.7.8.9:443"),
	}
	for _, s := range http {
		if s.FrontendPort != 80 {
			t.Fatalf("All http services should get multiplexed via the same frontend port: %+v", s)
		}
		expectedEps, ok := expectedURLMapping[s.Name]
		if !ok {
			t.Fatalf("Expected url endpoint %v, found %+v", s.Name, expectedURLMapping)
		}
		receivedEp := sets.NewString()
		for i := range s.Ep {
			receivedEp.Insert(s.Ep[i])
		}
		if len(receivedEp) != len(expectedEps) && !receivedEp.IsSuperset(expectedEps) {
			t.Fatalf("Expected %+v, got %+v", expectedEps, receivedEp)
		}
	}
}

func TestNewStaticPageHandler(t *testing.T) {
	defPagePath, _ := filepath.Abs("haproxy.cfg")
	defErrorPath, _ := filepath.Abs("template.cfg")
	defErrURL := "http://www.k8s.io"

	testDefPage := "file://" + defPagePath
	testErrorPage := "file://" + defErrorPath
	testReturnCode := 404

	handler := newStaticPageHandler("", testDefPage, testReturnCode)
	if handler == nil {
		t.Fatalf("Expected page handler")
	}

	handler = newStaticPageHandler(testErrorPage, testDefPage, testReturnCode)
	if handler.pagePath != testErrorPage {
		t.Fatalf("Expected local file content but got default page")
	}

	handler = newStaticPageHandler(defErrURL, testDefPage, testReturnCode)
	if handler.pagePath != defErrURL {
		t.Fatalf("Expected remote error page content but got default page")
	}

	handler = newStaticPageHandler(defErrURL+"s", testDefPage, 200)
	if handler.pagePath != testDefPage {
		t.Fatalf("Expected local file content with not valid URL")
	}
	if handler.returnCode != 200 {
		t.Fatalf("Expected a 200 return code.")
	}
}

//	buildTestLoadBalancer build a common loadBalancerController to be used
//	in the tests to verify the generated HAProxy configuration file
func buildTestLoadBalancer(lbDefAlgorithm string) *loadBalancerController {
	endpointAddresses := []api.EndpointAddress{
		{IP: "1.2.3.4"},
		{IP: "5.6.7.8"},
	}
	ports := []int{80, 443}
	endpointPorts := []api.EndpointPort{
		{Port: ports[0], Protocol: "TCP"},
		{Port: ports[1], Protocol: "HTTP"},
	}
	servicePorts := []api.ServicePort{
		{Port: ports[0], TargetPort: intstr.FromInt(ports[0])},
		{Port: ports[1], TargetPort: intstr.FromInt(ports[1])},
	}

	svc1 := getService(servicePorts)
	svc1.ObjectMeta.Name = "svc-1"
	svc2 := getService(servicePorts)
	svc2.ObjectMeta.Name = "svc-2"
	endpoints := []*api.Endpoints{
		getEndpoints(svc1, endpointAddresses, endpointPorts),
		getEndpoints(svc2, endpointAddresses, endpointPorts),
	}
	flb := newFakeLoadBalancerController(endpoints, []*api.Service{svc1, svc2})
	cfg, _ := filepath.Abs("./test-samples/loadbalancer_test.json")
	// do not have the input parameters. We need to specify a default.
	if lbDefAlgorithm == "" {
		lbDefAlgorithm = "roundrobin"
	}

	flb.cfg = parseCfg(cfg, lbDefAlgorithm, "", "")
	cfgFile, _ := filepath.Abs("test-" + string(util.NewUUID()))
	flb.cfg.Config = cfgFile
	flb.tcpServices = map[string]int{
		svc1.Name: 20,
	}

	return flb
}

// compareCfgFiles check that two files are equals
func compareCfgFiles(t *testing.T, orig, template string) {
	f1, err := ioutil.ReadFile(orig)
	if err != nil {
		t.Fatalf("Expected a file but an error was returned: %v", err)
	}
	f2, err := ioutil.ReadFile(template)
	if err != nil {
		t.Fatalf("Expected a file but an error was returned: %v", err)
	}

	if !bytes.Equal(f1, f2) {
		t.Fatalf("Expected the file contents were equals")
	}
}

func TestDefaultAlgorithm(t *testing.T) {
	flb := buildTestLoadBalancer("")
	httpSvc, _, tcpSvc := flb.getServices()
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected a valid HAProxy cfg, but an error was returned: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestDefaultAlgorithm.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestDefaultCustomAlgorithm(t *testing.T) {
	flb := buildTestLoadBalancer("leastconn")
	httpSvc, _, tcpSvc := flb.getServices()
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestDefaultCustomAlgorithm.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestSyslog(t *testing.T) {
	flb := buildTestLoadBalancer("")
	httpSvc, _, tcpSvc := flb.getServices()
	flb.cfg.startSyslog = true
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestSyslog.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestSvcCustomAlgorithm(t *testing.T) {
	flb := buildTestLoadBalancer("")
	httpSvc, _, tcpSvc := flb.getServices()
	httpSvc[0].Algorithm = "leastconn"
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestSvcCustomAlgorithm.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestCustomDefaultAndSvcAlgorithm(t *testing.T) {
	flb := buildTestLoadBalancer("leastconn")
	httpSvc, _, tcpSvc := flb.getServices()
	httpSvc[0].Algorithm = "roundrobin"
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestCustomDefaultAndSvcAlgorithm.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestServiceAffinity(t *testing.T) {
	flb := buildTestLoadBalancer("")
	httpSvc, _, tcpSvc := flb.getServices()
	httpSvc[0].SessionAffinity = true
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestServiceAffinity.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}

func TestServiceAffinityWithCookies(t *testing.T) {
	flb := buildTestLoadBalancer("")
	httpSvc, _, tcpSvc := flb.getServices()
	httpSvc[0].SessionAffinity = true
	httpSvc[0].CookieStickySession = true
	if err := flb.cfg.write(
		map[string][]service{
			"http": httpSvc,
			"tcp":  tcpSvc,
		}, false); err != nil {
		t.Fatalf("Expected at least one tcp or http service: %v", err)
	}
	template, _ := filepath.Abs("./test-samples/TestServiceAffinityWithCookies.cfg")
	compareCfgFiles(t, flb.cfg.Config, template)
	os.Remove(flb.cfg.Config)
}
