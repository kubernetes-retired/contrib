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
			Upstream{
				Name: "nginxApp-localhost-helloApp-80",
				UpstreamServer: UpstreamServer{
					Address: "10.0.0.1",
					Port:    "80",
				},
			},
		},
		Servers: []Server{
			Server{
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
