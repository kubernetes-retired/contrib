/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package kubectl

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		params    map[string]interface{}
		expected  *api.ReplicationController
		expectErr bool
	}{
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"port":     "-1",
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"run": "foo"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"run": "foo"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"run": "foo"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  "foo",
									Image: "someimage",
								},
							},
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"port":     "-1",
				"args":     []string{"bar", "baz", "blah"},
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"run": "foo"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"run": "foo"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"run": "foo"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  "foo",
									Image: "someimage",
									Args:  []string{"bar", "baz", "blah"},
								},
							},
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"port":     "-1",
				"args":     []string{"bar", "baz", "blah"},
				"command":  "true",
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"run": "foo"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"run": "foo"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"run": "foo"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:    "foo",
									Image:   "someimage",
									Command: []string{"bar", "baz", "blah"},
								},
							},
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"port":     "80",
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"run": "foo"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"run": "foo"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"run": "foo"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  "foo",
									Image: "someimage",
									Ports: []api.ContainerPort{
										{
											ContainerPort: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"port":     "80",
				"hostport": "80",
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"run": "foo"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"run": "foo"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"run": "foo"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  "foo",
									Image: "someimage",
									Ports: []api.ContainerPort{
										{
											ContainerPort: 80,
											HostPort:      80,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"hostport": "80",
			},
			expected:  nil,
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"labels":   "foo=bar,baz=blah",
			},
			expected: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"foo": "bar", "baz": "blah"},
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
					Selector: map[string]string{"foo": "bar", "baz": "blah"},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"foo": "bar", "baz": "blah"},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  "foo",
									Image: "someimage",
								},
							},
						},
					},
				},
			},
		},
	}
	generator := BasicReplicationController{}
	for _, test := range tests {
		obj, err := generator.Generate(test.params)
		if !test.expectErr && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if test.expectErr && err != nil {
			continue
		}
		if !reflect.DeepEqual(obj.(*api.ReplicationController).Spec.Template, test.expected.Spec.Template) {
			t.Errorf("\nexpected:\n%#v\nsaw:\n%#v", test.expected.Spec.Template, obj.(*api.ReplicationController).Spec.Template)
		}
	}
}

func TestGeneratePod(t *testing.T) {
	tests := []struct {
		params    map[string]interface{}
		expected  *api.Pod
		expectErr bool
	}{
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": "someimage",
				"port":  "-1",
			},
			expected: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name: "foo",
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "foo",
							Image:           "someimage",
							ImagePullPolicy: api.PullIfNotPresent,
						},
					},
					DNSPolicy:     api.DNSClusterFirst,
					RestartPolicy: api.RestartPolicyAlways,
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": "someimage",
				"port":  "80",
			},
			expected: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name: "foo",
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "foo",
							Image:           "someimage",
							ImagePullPolicy: api.PullIfNotPresent,
							Ports: []api.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
					DNSPolicy:     api.DNSClusterFirst,
					RestartPolicy: api.RestartPolicyAlways,
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"port":     "80",
				"hostport": "80",
			},
			expected: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name: "foo",
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "foo",
							Image:           "someimage",
							ImagePullPolicy: api.PullIfNotPresent,
							Ports: []api.ContainerPort{
								{
									ContainerPort: 80,
									HostPort:      80,
								},
							},
						},
					},
					DNSPolicy:     api.DNSClusterFirst,
					RestartPolicy: api.RestartPolicyAlways,
				},
			},
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"hostport": "80",
			},
			expected:  nil,
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name":     "foo",
				"image":    "someimage",
				"replicas": "1",
				"labels":   "foo=bar,baz=blah",
			},
			expected: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"foo": "bar", "baz": "blah"},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "foo",
							Image:           "someimage",
							ImagePullPolicy: api.PullIfNotPresent,
						},
					},
					DNSPolicy:     api.DNSClusterFirst,
					RestartPolicy: api.RestartPolicyAlways,
				},
			},
		},
	}
	generator := BasicPod{}
	for _, test := range tests {
		obj, err := generator.Generate(test.params)
		if !test.expectErr && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if test.expectErr && err != nil {
			continue
		}
		if !reflect.DeepEqual(obj.(*api.Pod), test.expected) {
			t.Errorf("\nexpected:\n%#v\nsaw:\n%#v", test.expected, obj.(*api.Pod))
		}
	}
}
