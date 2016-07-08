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
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

const (
	UserAgent = "terraform-kubernetes"

	PollInterval = 5 * time.Second
	PollTimeout  = 5 * time.Minute
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"kubernetes_kubeconfig": resourceKubeconfig(),
		},
	}
}

func resourceKubeconfig() *schema.Resource {
	return &schema.Resource{
		Create: CreateKubeconfig,
		Delete: DeleteKubeconfig,
		Read:   ReadKubeconfig,

		Schema: map[string]*schema.Schema{
			"server": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Domain name or IP address of the API server",
				ForceNew:    true,
			},
			"configdata": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "kubeconfig in the serialized JSON format",
				ForceNew:    true,
			},
		},
	}
}

func CreateKubeconfig(d *schema.ResourceData, meta interface{}) error {
	server := d.Get("server").(string)
	cfg, err := clientcmd.BuildConfigFromKubeconfigGetter(server, kubeCfgGetter(d))
	if err != nil {
		return fmt.Errorf("couldn't parse the supplied config: %v", err)
	}
	clientset, err := release_1_4.NewForConfig(restclient.AddUserAgent(cfg, UserAgent))
	if err != nil {
		return fmt.Errorf("failed to initialize the cluster client: %v", err)
	}

	interval := time.NewTicker(PollInterval)
	defer interval.Stop()
	timeout := time.NewTimer(PollTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-interval.C:
			if allComponentsHealthy(clientset) {
				break
			}
		case <-timeout.C:
			return fmt.Errorf("cluster components never turned healthy")
		}
	}

	return nil
}

func DeleteKubeconfig(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func ReadKubeconfig(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func allComponentsHealthy(clientset *release_1_4.Clientset) bool {
	csList, err := clientset.Core().ComponentStatuses().List(api.ListOptions{})
	if err != nil || len(csList.Items) <= 0 {
		return false
	}
	for _, cs := range csList.Items {
		if !(len(cs.Conditions) > 1 && cs.Conditions[0].Type == "Healthy" && cs.Conditions[0].Status == "True") {
			return false
		}
	}
	return true
}

func kubeCfgGetter(d *schema.ResourceData) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		kubeCfgStr := d.Get("configdata").(string)
		return clientcmd.Load([]byte(kubeCfgStr))
	}
}
