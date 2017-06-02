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

package nginx

import (
	"reflect"
	"testing"

	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
)

func TestGetNGINXConfigFileName(t *testing.T) {
	path := "path"
	name := "filename"
	expected := "path/filename.conf"
	if result := getNGINXConfigFileName(path, name); result != expected {
		t.Errorf("getNGINXConfigFileName(%q, %q) returned %q. Expected %q. ", path, name, result, expected)
	}
}

func TestGenerateNGINXCfg(t *testing.T) {
	certPath := "/etc/nginx/ssl"
	name := "nginxApp"
	configObject := generateExampleBackendObject()

	expectedNginxConfig := NGINXConfig{
		Upstreams: []Upstream{
			{
				Name: "nginxApp-localhost-helloApp-80",
				UpstreamServer: UpstreamServer{
					Address: "10.0.0.1",
					Port:    "80",
				},
			},
		},
		Servers: []Server{
			{
				Name:     "localhost",
				BindIP:   "127.0.0.1",
				BindPort: "80",
				Location: Location{
					Path: "/hello",
					Upstream: Upstream{
						Name: "nginxApp-localhost-helloApp-80",
						UpstreamServer: UpstreamServer{
							Address: "10.0.0.1",
							Port:    "80",
						},
					},
				},
			},
		},
	}

	nginxConfig := generateNGINXCfg(certPath, name, configObject)
	if !reflect.DeepEqual(nginxConfig, expectedNginxConfig) {
		t.Error(
			"In generateNGINXCfg()",
			"expected", expectedNginxConfig,
			"got", nginxConfig,
		)
	}
}

func generateExampleBackendObject() factory.BackendConfig {
	backendConfig := factory.BackendConfig{
		Host:              "localhost",
		Namespace:         "default",
		BindIp:            "127.0.0.1",
		Ports:             []string{"80"},
		TargetServiceName: "helloApp",
		TargetIP:          "10.0.0.1",
		SSL:               false,
		Path:              "/hello",
	}
	return backendConfig
}
