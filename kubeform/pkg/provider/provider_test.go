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

package provider

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4/fake"
)

var testProviders map[string]terraform.ResourceProvider
var testProvider *schema.Provider

func init() {
	testProvider = Provider().(*schema.Provider)
	testProvider.ConfigureFunc = testProviderConfig
	testProviders = map[string]terraform.ResourceProvider{
		"kubernetes": testProvider,
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().(*schema.Provider).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestClusterHealthy(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset(defaultClusterComponents())
	if !poll(1*time.Second, 10*time.Second, allComponentsHealthy(fakeClientset)) {
		t.Errorf("Expected cluster to be healthy, got failure")
	}
}

func TestCluster(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testPreCheck(t) },
		Providers:    testProviders,
		CheckDestroy: testClusterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testClusterBasic(),
				Check: resource.ComposeTestCheckFunc(
					testClusterExists("kubernetes_cluster.cluster-foo"),
				),
			},
		},
	})
}

func defaultClusterComponents() *v1.ComponentStatusList {
	healthyCond := []v1.ComponentCondition{
		{
			Type:   "Healthy",
			Status: "True",
		},
	}
	return &v1.ComponentStatusList{
		Items: []v1.ComponentStatus{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "c1",
				},
				Conditions: healthyCond,
			},
		},
	}
}

func testProviderConfig(d *schema.ResourceData) (interface{}, error) {
	var f configFunc = func(d *schema.ResourceData) (*config, error) {
		nodes := &v1.NodeList{
			Items: []v1.Node{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "n1",
					},
				},
			},
		}
		return &config{
			pollInterval:             1 * time.Millisecond,
			pollTimeout:              2 * time.Millisecond,
			configPollInterval:       1 * time.Millisecond,
			ConfigPollTimeout:        2 * time.Millisecond,
			resourceShutdownInterval: 0 * time.Second,

			kubeConfig: nil,
			clientset:  fake.NewSimpleClientset(defaultClusterComponents(), nodes),
		}, nil
	}
	return f, nil
}

func testPreCheck(t *testing.T) {
	log.Printf("[DEBUG] testPreCheck called")
	return
}

func testClusterBasic() string {
	return fmt.Sprintf(`
	resource "kubernetes_cluster" "cluster-foo" {
		server = "https://cluster-master.test"
		configdata = "{}"
	}`)
}

func testClusterDestroy(s *terraform.State) error {
	log.Printf("[DEBUG] testClusterDestroy called")
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kubernetes_cluster" {
			continue
		}

		// _, err := config.clientCompute.Firewalls.Get(
		// 	config.Project, rs.Primary.ID).Do()
		// if err == nil {
		// 	return fmt.Errorf("Firewall still exists")
		// }
	}

	return nil
}

func testClusterExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		log.Printf("[DEBUG] testClusterExists called")
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set")
		}

		return nil
	}
}
