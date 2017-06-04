/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kubernetes

import (
	"fmt"

	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	flag "github.com/spf13/pflag"
)

// CreateKubeClient returns kubernetes client.
func CreateKubeClient(flags *flag.FlagSet, inCluster bool) (*kube_client.Client, error) {
	if inCluster {
		return kube_client.NewInCluster()
	} else {
		clientConfig := kubectl_util.DefaultClientConfig(flags)
		config, err := clientConfig.ClientConfig()
		if err != nil {
			fmt.Errorf("error connecting to the client: %v", err)
		}
		return kube_client.NewOrDie(config), nil
	}
}
