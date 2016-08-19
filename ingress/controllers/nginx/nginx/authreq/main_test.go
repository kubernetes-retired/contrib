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

package authreq

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/intstr"
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

	_, err := ingAnnotations(ing.GetAnnotations()).url()
	if err == nil {
		t.Error("Expected a validation error")
	}

	data := map[string]string{}
	ing.SetAnnotations(data)

	tests := []struct {
		title  string
		url    string
		expErr bool
	}{
		{"empty", "", true},
		{"no scheme", "bar", true},
		{"invalid host", "http://", true},
		{"invalid host (multiple dots)", "http://foo..bar.com", true},
		{"valid URL", "http://bar.foo.com/external-auth", false},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			data[authURL] = test.url
			u, err := ParseAnnotations(ing)

			if test.expErr && err == nil {
				t.Errorf("expected error but retuned nil")
			}

			if !test.expErr && u != test.url {
				t.Errorf("expected \"%v\" but \"%v\" was returned", test.url, u)
			}
		})
	}
}
