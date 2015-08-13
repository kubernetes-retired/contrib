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

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

type internalType struct {
	Kind       string
	APIVersion string

	Name string
}

type externalType struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`

	Name string `json:"name"`
}

type ExternalType2 struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`

	Name string `json:"name"`
}

func (*internalType) IsAnAPIObject()  {}
func (*externalType) IsAnAPIObject()  {}
func (*ExternalType2) IsAnAPIObject() {}

var versionErr = errors.New("not a version")

func versionErrIfFalse(b bool) error {
	if b {
		return nil
	}
	return versionErr
}

func newExternalScheme() (*runtime.Scheme, meta.RESTMapper, runtime.Codec) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName("", "Type", &internalType{})
	scheme.AddKnownTypeWithName("unlikelyversion", "Type", &externalType{})
	//This tests that kubectl will not confuse the external scheme with the internal scheme, even when they accidentally have versions of the same name.
	scheme.AddKnownTypeWithName(testapi.Version(), "Type", &ExternalType2{})

	codec := runtime.CodecFor(scheme, "unlikelyversion")
	validVersion := testapi.Version()
	mapper := meta.NewDefaultRESTMapper("apitest", []string{"unlikelyversion", validVersion}, func(version string) (*meta.VersionInterfaces, error) {
		return &meta.VersionInterfaces{
			Codec:            runtime.CodecFor(scheme, version),
			ObjectConvertor:  scheme,
			MetadataAccessor: meta.NewAccessor(),
		}, versionErrIfFalse(version == validVersion || version == "unlikelyversion")
	})
	for _, version := range []string{"unlikelyversion", validVersion} {
		for kind := range scheme.KnownTypes(version) {
			mixedCase := false
			scope := meta.RESTScopeNamespace
			mapper.Add(scope, kind, version, mixedCase)
		}
	}

	return scheme, mapper, codec
}

type testPrinter struct {
	Objects []runtime.Object
	Err     error
}

func (t *testPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	t.Objects = append(t.Objects, obj)
	fmt.Fprintf(out, "%#v", obj)
	return t.Err
}

type testDescriber struct {
	Name, Namespace string
	Output          string
	Err             error
}

func (t *testDescriber) Describe(namespace, name string) (output string, err error) {
	t.Namespace, t.Name = namespace, name
	return t.Output, t.Err
}

type testFactory struct {
	Mapper       meta.RESTMapper
	Typer        runtime.ObjectTyper
	Client       kubectl.RESTClient
	Describer    kubectl.Describer
	Printer      kubectl.ResourcePrinter
	Validator    validation.Schema
	Namespace    string
	ClientConfig *client.Config
	Err          error
}

func NewTestFactory() (*cmdutil.Factory, *testFactory, runtime.Codec) {
	scheme, mapper, codec := newExternalScheme()
	t := &testFactory{
		Validator: validation.NullSchema{},
		Mapper:    mapper,
		Typer:     scheme,
	}
	return &cmdutil.Factory{
		Object: func() (meta.RESTMapper, runtime.ObjectTyper) {
			return t.Mapper, t.Typer
		},
		RESTClient: func(*meta.RESTMapping) (resource.RESTClient, error) {
			return t.Client, t.Err
		},
		Describer: func(*meta.RESTMapping) (kubectl.Describer, error) {
			return t.Describer, t.Err
		},
		Printer: func(mapping *meta.RESTMapping, noHeaders, withNamespace bool, wide bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
			return t.Printer, t.Err
		},
		Validator: func() (validation.Schema, error) {
			return t.Validator, t.Err
		},
		DefaultNamespace: func() (string, bool, error) {
			return t.Namespace, false, t.Err
		},
		ClientConfig: func() (*client.Config, error) {
			return t.ClientConfig, t.Err
		},
	}, t, codec
}

func NewMixedFactory(apiClient resource.RESTClient) (*cmdutil.Factory, *testFactory, runtime.Codec) {
	f, t, c := NewTestFactory()
	f.Object = func() (meta.RESTMapper, runtime.ObjectTyper) {
		return meta.MultiRESTMapper{t.Mapper, latest.RESTMapper}, runtime.MultiObjectTyper{t.Typer, api.Scheme}
	}
	f.RESTClient = func(m *meta.RESTMapping) (resource.RESTClient, error) {
		if m.ObjectConvertor == api.Scheme {
			return apiClient, t.Err
		}
		return t.Client, t.Err
	}
	return f, t, c
}

func NewAPIFactory() (*cmdutil.Factory, *testFactory, runtime.Codec) {
	t := &testFactory{
		Validator: validation.NullSchema{},
	}
	generators := map[string]kubectl.Generator{
		"run/v1":     kubectl.BasicReplicationController{},
		"service/v1": kubectl.ServiceGeneratorV1{},
		"service/v2": kubectl.ServiceGeneratorV2{},
	}
	return &cmdutil.Factory{
		Object: func() (meta.RESTMapper, runtime.ObjectTyper) {
			return latest.RESTMapper, api.Scheme
		},
		Client: func() (*client.Client, error) {
			// Swap out the HTTP client out of the client with the fake's version.
			fakeClient := t.Client.(*client.FakeRESTClient)
			c := client.NewOrDie(t.ClientConfig)
			c.Client = fakeClient.Client
			return c, t.Err
		},
		RESTClient: func(*meta.RESTMapping) (resource.RESTClient, error) {
			return t.Client, t.Err
		},
		Describer: func(*meta.RESTMapping) (kubectl.Describer, error) {
			return t.Describer, t.Err
		},
		Printer: func(mapping *meta.RESTMapping, noHeaders, withNamespace bool, wide bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
			return t.Printer, t.Err
		},
		Validator: func() (validation.Schema, error) {
			return t.Validator, t.Err
		},
		DefaultNamespace: func() (string, bool, error) {
			return t.Namespace, false, t.Err
		},
		ClientConfig: func() (*client.Config, error) {
			return t.ClientConfig, t.Err
		},
		Generator: func(name string) (kubectl.Generator, bool) {
			generator, ok := generators[name]
			return generator, ok
		},
	}, t, testapi.Codec()
}

func objBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func stringBody(body string) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(body)))
}

// TODO(jlowdermilk): refactor the Factory so we can test client versions properly,
// with different client/server version skew scenarios.
// Verify that resource.RESTClients constructed from a factory respect mapping.APIVersion
//func TestClientVersions(t *testing.T) {
//	f := cmdutil.NewFactory(nil)
//
//	version := testapi.Version()
//	mapping := &meta.RESTMapping{
//		APIVersion: version,
//	}
//	c, err := f.RESTClient(mapping)
//	if err != nil {
//		t.Errorf("unexpected error: %v", err)
//	}
//	client := c.(*client.RESTClient)
//	if client.APIVersion() != version {
//		t.Errorf("unexpected Client APIVersion: %s %v", client.APIVersion, client)
//	}
//}

func ExamplePrintReplicationControllerWithNamespace() {
	f, tf, codec := NewAPIFactory()
	tf.Printer = kubectl.NewHumanReadablePrinter(false, true, false, []string{})
	tf.Client = &client.FakeRESTClient{
		Codec:  codec,
		Client: nil,
	}
	cmd := NewCmdRun(f, os.Stdin, os.Stdout, os.Stderr)
	ctrl := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name:              "foo",
			Namespace:         "beep",
			Labels:            map[string]string{"foo": "bar"},
			CreationTimestamp: util.Time{Time: time.Now().AddDate(-10, 0, 0)},
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
			Selector: map[string]string{"foo": "bar"},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
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
	}
	err := f.PrintObject(cmd, ctrl, os.Stdout)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	// Output:
	// NAMESPACE   CONTROLLER   CONTAINER(S)   IMAGE(S)    SELECTOR   REPLICAS   AGE
	// beep        foo          foo            someimage   foo=bar    1          10y
}

func ExamplePrintPodWithWideFormat() {
	f, tf, codec := NewAPIFactory()
	tf.Printer = kubectl.NewHumanReadablePrinter(false, false, true, []string{})
	tf.Client = &client.FakeRESTClient{
		Codec:  codec,
		Client: nil,
	}
	nodeName := "kubernetes-minion-abcd"
	cmd := NewCmdRun(f, os.Stdin, os.Stdout, os.Stderr)
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:              "test1",
			CreationTimestamp: util.Time{Time: time.Now().AddDate(-10, 0, 0)},
		},
		Spec: api.PodSpec{
			Containers: make([]api.Container, 2),
			NodeName:   nodeName,
		},
		Status: api.PodStatus{
			Phase: "podPhase",
			ContainerStatuses: []api.ContainerStatus{
				{Ready: true, RestartCount: 3, State: api.ContainerState{Running: &api.ContainerStateRunning{}}},
				{RestartCount: 3},
			},
		},
	}
	err := f.PrintObject(cmd, pod, os.Stdout)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	// Output:
	// NAME      READY     STATUS     RESTARTS   AGE       NODE
	// test1     1/2       podPhase   6          10y       kubernetes-minion-abcd
}

func ExamplePrintServiceWithNamespacesAndLabels() {
	f, tf, codec := NewAPIFactory()
	tf.Printer = kubectl.NewHumanReadablePrinter(false, true, false, []string{"l1"})
	tf.Client = &client.FakeRESTClient{
		Codec:  codec,
		Client: nil,
	}
	cmd := NewCmdRun(f, os.Stdin, os.Stdout, os.Stderr)
	svc := &api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{
					Name:              "svc1",
					Namespace:         "ns1",
					CreationTimestamp: util.Time{Time: time.Now().AddDate(-10, 0, 0)},
					Labels: map[string]string{
						"l1": "value",
					},
				},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{Protocol: "UDP", Port: 53},
						{Protocol: "TCP", Port: 53},
					},
					Selector: map[string]string{
						"s": "magic",
					},
					ClusterIP: "10.1.1.1",
				},
				Status: api.ServiceStatus{},
			},
			{
				ObjectMeta: api.ObjectMeta{
					Name:              "svc2",
					Namespace:         "ns2",
					CreationTimestamp: util.Time{Time: time.Now().AddDate(-10, 0, 0)},
					Labels: map[string]string{
						"l1": "dolla-bill-yall",
					},
				},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{Protocol: "TCP", Port: 80},
						{Protocol: "TCP", Port: 8080},
					},
					Selector: map[string]string{
						"s": "kazam",
					},
					ClusterIP: "10.1.1.2",
				},
				Status: api.ServiceStatus{},
			}},
	}
	ld := util.NewLineDelimiter(os.Stdout, "|")
	defer ld.Flush()
	err := f.PrintObject(cmd, svc, ld)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	// Output:
	// |NAMESPACE   NAME      CLUSTER_IP   EXTERNAL_IP   PORT(S)           SELECTOR   AGE       L1|
	// |ns1         svc1      10.1.1.1     unknown       53/UDP,53/TCP     s=magic    10y       value|
	// |ns2         svc2      10.1.1.2     unknown       80/TCP,8080/TCP   s=kazam    10y       dolla-bill-yall|
	// ||
}

func TestNormalizationFuncGlobalExistence(t *testing.T) {
	// This test can be safely deleted when we will not support multiple flag formats
	root := NewKubectlCommand(cmdutil.NewFactory(nil), os.Stdin, os.Stdout, os.Stderr)

	if root.Parent() != nil {
		t.Fatal("We expect the root command to be returned")
	}
	if root.GlobalNormalizationFunc() == nil {
		t.Fatal("We expect that root command has a global normalization function")
	}

	if reflect.ValueOf(root.GlobalNormalizationFunc()).Pointer() != reflect.ValueOf(root.Flags().GetNormalizeFunc()).Pointer() {
		t.Fatal("root command seems to have a wrong normalization function")
	}

	sub := root
	for sub.HasSubCommands() {
		sub = sub.Commands()[0]
	}

	// In case of failure of this test check this PR: spf13/cobra#110
	if reflect.ValueOf(sub.Flags().GetNormalizeFunc()).Pointer() != reflect.ValueOf(root.Flags().GetNormalizeFunc()).Pointer() {
		t.Fatal("child and root commands should have the same normalization functions")
	}
}
