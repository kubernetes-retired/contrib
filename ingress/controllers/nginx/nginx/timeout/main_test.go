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

package timeout

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/intstr"

	"k8s.io/contrib/ingress/controllers/nginx/nginx/config"
)

func buildIngress() *extensions.Ingress {
	defaultBackend := extensions.IngressBackend{
		ServiceName: "default-backend",
		ServicePort: intstr.FromInt(80),
	}

	return &extensions.Ingress{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: extensions.IngressSpec{
			Backend: &extensions.IngressBackend{
				ServiceName: "default-backend",
				ServicePort: intstr.FromInt(80),
			},
			Rules: []extensions.IngressRule{
				{
					Host: "foo.bar.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path:    "/foo",
									Backend: defaultBackend,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestAnnotations(t *testing.T) {
	ing := buildIngress()

	_, err := ingAnnotations(ing.GetAnnotations()).connectTimeout()
	if err == nil {
		t.Error("Expected a validation error")
	}

	_, err = ingAnnotations(ing.GetAnnotations()).readTimeout()
	if err == nil {
		t.Error("Expected a validation error")
	}

	_, err = ingAnnotations(ing.GetAnnotations()).sendTimeout()
	if err == nil {
		t.Error("Expected a validation error")
	}

	data := map[string]string{}
	data[proxyConnectTimeout] = "9"
	data[proxyReadTimeout] = "10"
	data[proxySendTimeout] = "11"
	ing.SetAnnotations(data)

	pct, err := ingAnnotations(ing.GetAnnotations()).connectTimeout()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if pct != 9 {
		t.Errorf("Expected 9 but returned %v", pct)
	}

	prt, err := ingAnnotations(ing.GetAnnotations()).readTimeout()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if prt != 10 {
		t.Errorf("Expected 10 but returned %v", prt)
	}

	pst, err := ingAnnotations(ing.GetAnnotations()).sendTimeout()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if pst != 11 {
		t.Errorf("Expected 11 but returned %v", pst)
	}
}

func TestIngressTimeout(t *testing.T) {
	ing := buildIngress()

	data := map[string]string{}
	data[proxyConnectTimeout] = "9"
	data[proxyReadTimeout] = "10"
	data[proxySendTimeout] = "11"
	ing.SetAnnotations(data)

	cfg := config.Configuration{}
	cfg.ProxyConnectTimeout = 19
	cfg.ProxyReadTimeout = 20
	cfg.ProxySendTimeout = 21

	timeout := ParseAnnotations(cfg, ing)

	if timeout.connect != 9 {
		t.Errorf("Expected 9 as connect-timeout but returned %v", timeout.connect)
	}

	if timeout.read != 10 {
		t.Errorf("Expected 10 as connect-timeout but returned %v", timeout.read)
	}

	if timeout.send != 11 {
		t.Errorf("Expected 11 as connect-timeout but returned %v", timeout.send)
	}
}
