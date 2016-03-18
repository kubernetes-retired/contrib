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

package util

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful/swagger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer/json"
	"k8s.io/kubernetes/pkg/util"
)

const (
	FlagMatchBinaryVersion = "match-server-version"
)

// Factory provides abstractions that allow the Kubectl command to be extended across multiple types
// of resources and different API sets.
// TODO: make the functions interfaces
// TODO: pass the various interfaces on the factory directly into the command constructors (so the
// commands are decoupled from the factory).
type Factory struct {
	clients *ClientCache
	flags   *pflag.FlagSet
	cmd     string

	// Returns interfaces for dealing with arbitrary runtime.Objects.
	Object func() (meta.RESTMapper, runtime.ObjectTyper)
	// Returns interfaces for decoding objects - if toInternal is set, decoded objects will be converted
	// into their internal form (if possible). Eventually the internal form will be removed as an option,
	// and only versioned objects will be returned.
	Decoder func(toInternal bool) runtime.Decoder
	// Returns an encoder capable of encoding a provided object into JSON in the default desired version.
	JSONEncoder func() runtime.Encoder
	// Returns a client for accessing Kubernetes resources or an error.
	Client func() (*client.Client, error)
	// Returns a client.Config for accessing the Kubernetes server.
	ClientConfig func() (*client.Config, error)
	// Returns a RESTClient for working with the specified RESTMapping or an error. This is intended
	// for working with arbitrary resources and is not guaranteed to point to a Kubernetes APIServer.
	ClientForMapping func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	// Returns a Describer for displaying the specified RESTMapping type or an error.
	Describer func(mapping *meta.RESTMapping) (kubectl.Describer, error)
	// Returns a Printer for formatting objects of the given type or an error.
	Printer func(mapping *meta.RESTMapping, noHeaders, withNamespace bool, wide bool, showAll bool, showLabels bool, absoluteTimestamps bool, columnLabels []string) (kubectl.ResourcePrinter, error)
	// Returns a Scaler for changing the size of the specified RESTMapping type or an error
	Scaler func(mapping *meta.RESTMapping) (kubectl.Scaler, error)
	// Returns a Reaper for gracefully shutting down resources.
	Reaper func(mapping *meta.RESTMapping) (kubectl.Reaper, error)
	// Returns a HistoryViewer for viewing change history
	HistoryViewer func(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error)
	// Returns a Rollbacker for changing the rollback version of the specified RESTMapping type or an error
	Rollbacker func(mapping *meta.RESTMapping) (kubectl.Rollbacker, error)
	// PodSelectorForObject returns the pod selector associated with the provided object
	PodSelectorForObject func(object runtime.Object) (string, error)
	// MapBasedSelectorForObject returns the map-based selector associated with the provided object. If a
	// new set-based selector is provided, an error is returned if the selector cannot be converted to a
	// map-based selector
	MapBasedSelectorForObject func(object runtime.Object) (string, error)
	// PortsForObject returns the ports associated with the provided object
	PortsForObject func(object runtime.Object) ([]string, error)
	// LabelsForObject returns the labels associated with the provided object
	LabelsForObject func(object runtime.Object) (map[string]string, error)
	// LogsForObject returns a request for the logs associated with the provided object
	LogsForObject func(object, options runtime.Object) (*client.Request, error)
	// PauseObject marks the provided object as paused ie. it will not be reconciled by its controller.
	PauseObject func(object runtime.Object) (bool, error)
	// ResumeObject resumes a paused object ie. it will be reconciled by its controller.
	ResumeObject func(object runtime.Object) (bool, error)
	// Returns a schema that can validate objects stored on disk.
	Validator func(validate bool, cacheDir string) (validation.Schema, error)
	// SwaggerSchema returns the schema declaration for the provided group version kind.
	SwaggerSchema func(unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error)
	// Returns the default namespace to use in cases where no
	// other namespace is specified and whether the namespace was
	// overriden.
	DefaultNamespace func() (string, bool, error)
	// Generators returns the generators for the provided command
	Generators func(cmdName string) map[string]kubectl.Generator
	// Check whether the kind of resources could be exposed
	CanBeExposed func(kind unversioned.GroupKind) error
	// Check whether the kind of resources could be autoscaled
	CanBeAutoscaled func(kind unversioned.GroupKind) error
	// AttachablePodForObject returns the pod to which to attach given an object.
	AttachablePodForObject func(object runtime.Object) (*api.Pod, error)
	// EditorEnvs returns a group of environment variables that the edit command
	// can range over in order to determine if the user has specified an editor
	// of their choice.
	EditorEnvs func() []string
}

const (
	RunV1GeneratorName                          = "run/v1"
	RunPodV1GeneratorName                       = "run-pod/v1"
	ServiceV1GeneratorName                      = "service/v1"
	ServiceV2GeneratorName                      = "service/v2"
	HorizontalPodAutoscalerV1Beta1GeneratorName = "horizontalpodautoscaler/v1beta1"
	DeploymentV1Beta1GeneratorName              = "deployment/v1beta1"
	JobV1Beta1GeneratorName                     = "job/v1beta1"
	NamespaceV1GeneratorName                    = "namespace/v1"
	SecretV1GeneratorName                       = "secret/v1"
	SecretForDockerRegistryV1GeneratorName      = "secret-for-docker-registry/v1"
	ConfigMapV1GeneratorName                    = "configmap/v1"
)

// DefaultGenerators returns the set of default generators for use in Factory instances
func DefaultGenerators(cmdName string) map[string]kubectl.Generator {
	generators := map[string]map[string]kubectl.Generator{}
	generators["expose"] = map[string]kubectl.Generator{
		ServiceV1GeneratorName: kubectl.ServiceGeneratorV1{},
		ServiceV2GeneratorName: kubectl.ServiceGeneratorV2{},
	}
	generators["run"] = map[string]kubectl.Generator{
		RunV1GeneratorName:             kubectl.BasicReplicationController{},
		RunPodV1GeneratorName:          kubectl.BasicPod{},
		DeploymentV1Beta1GeneratorName: kubectl.DeploymentV1Beta1{},
		JobV1Beta1GeneratorName:        kubectl.JobV1Beta1{},
	}
	generators["autoscale"] = map[string]kubectl.Generator{
		HorizontalPodAutoscalerV1Beta1GeneratorName: kubectl.HorizontalPodAutoscalerV1Beta1{},
	}
	generators["namespace"] = map[string]kubectl.Generator{
		NamespaceV1GeneratorName: kubectl.NamespaceGeneratorV1{},
	}
	generators["secret"] = map[string]kubectl.Generator{
		SecretV1GeneratorName: kubectl.SecretGeneratorV1{},
	}
	generators["secret-for-docker-registry"] = map[string]kubectl.Generator{
		SecretForDockerRegistryV1GeneratorName: kubectl.SecretForDockerRegistryGeneratorV1{},
	}
	return generators[cmdName]
}

// NewFactory creates a factory with the default Kubernetes resources defined
// if optionalClientConfig is nil, then flags will be bound to a new clientcmd.ClientConfig.
// if optionalClientConfig is not nil, then this factory will make use of it.
func NewFactory(optionalClientConfig clientcmd.ClientConfig) *Factory {
	mapper := kubectl.ShortcutExpander{RESTMapper: api.RESTMapper}

	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SetNormalizeFunc(util.WarnWordSepNormalizeFunc) // Warn for "_" flags

	clientConfig := optionalClientConfig
	if optionalClientConfig == nil {
		clientConfig = DefaultClientConfig(flags)
	}

	clients := NewClientCache(clientConfig)

	return &Factory{
		clients: clients,
		flags:   flags,
		cmd:     recordCommand(os.Args),

		Object: func() (meta.RESTMapper, runtime.ObjectTyper) {
			cfg, err := clientConfig.ClientConfig()
			CheckErr(err)
			cmdApiVersion := unversioned.GroupVersion{}
			if cfg.GroupVersion != nil {
				cmdApiVersion = *cfg.GroupVersion
			}

			return kubectl.OutputVersionMapper{RESTMapper: mapper, OutputVersions: []unversioned.GroupVersion{cmdApiVersion}}, api.Scheme
		},
		Client: func() (*client.Client, error) {
			return clients.ClientForVersion(nil)
		},
		ClientConfig: func() (*client.Config, error) {
			return clients.ClientConfigForVersion(nil)
		},
		ClientForMapping: func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			if err != nil {
				return nil, err
			}
			switch mapping.GroupVersionKind.Group {
			case api.GroupName:
				return client.RESTClient, nil
			case autoscaling.GroupName:
				return client.AutoscalingClient.RESTClient, nil
			case extensions.GroupName:
				return client.ExtensionsClient.RESTClient, nil
			}
			return nil, fmt.Errorf("unable to get RESTClient for resource '%s'", mapping.Resource)
		},
		Describer: func(mapping *meta.RESTMapping) (kubectl.Describer, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			if err != nil {
				return nil, err
			}
			if describer, ok := kubectl.DescriberFor(mapping.GroupVersionKind.GroupKind(), client); ok {
				return describer, nil
			}
			return nil, fmt.Errorf("no description has been implemented for %q", mapping.GroupVersionKind.Kind)
		},
		Decoder: func(toInternal bool) runtime.Decoder {
			if toInternal {
				return api.Codecs.UniversalDecoder()
			}
			return api.Codecs.UniversalDeserializer()
		},
		JSONEncoder: func() runtime.Encoder {
			return api.Codecs.LegacyCodec(registered.EnabledVersions()...)
		},
		Printer: func(mapping *meta.RESTMapping, noHeaders, withNamespace bool, wide bool, showAll bool, showLabels bool, absoluteTimestamps bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
			return kubectl.NewHumanReadablePrinter(noHeaders, withNamespace, wide, showAll, showLabels, absoluteTimestamps, columnLabels), nil
		},
		PodSelectorForObject: func(object runtime.Object) (string, error) {
			// TODO: replace with a swagger schema based approach (identify pod selector via schema introspection)
			switch t := object.(type) {
			case *api.ReplicationController:
				return kubectl.MakeLabels(t.Spec.Selector), nil
			case *api.Pod:
				if len(t.Labels) == 0 {
					return "", fmt.Errorf("the pod has no labels and cannot be exposed")
				}
				return kubectl.MakeLabels(t.Labels), nil
			case *api.Service:
				if t.Spec.Selector == nil {
					return "", fmt.Errorf("the service has no pod selector set")
				}
				return kubectl.MakeLabels(t.Spec.Selector), nil
			case *extensions.Deployment:
				selector, err := unversioned.LabelSelectorAsSelector(t.Spec.Selector)
				if err != nil {
					return "", fmt.Errorf("invalid label selector: %v", err)
				}
				return selector.String(), nil
			case *extensions.ReplicaSet:
				selector, err := unversioned.LabelSelectorAsSelector(t.Spec.Selector)
				if err != nil {
					return "", fmt.Errorf("failed to convert label selector to selector: %v", err)
				}
				return selector.String(), nil
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return "", err
				}
				return "", fmt.Errorf("cannot extract pod selector from %v", gvk)
			}
		},
		MapBasedSelectorForObject: func(object runtime.Object) (string, error) {
			// TODO: replace with a swagger schema based approach (identify pod selector via schema introspection)
			switch t := object.(type) {
			case *api.ReplicationController:
				return kubectl.MakeLabels(t.Spec.Selector), nil
			case *api.Pod:
				if len(t.Labels) == 0 {
					return "", fmt.Errorf("the pod has no labels and cannot be exposed")
				}
				return kubectl.MakeLabels(t.Labels), nil
			case *api.Service:
				if t.Spec.Selector == nil {
					return "", fmt.Errorf("the service has no pod selector set")
				}
				return kubectl.MakeLabels(t.Spec.Selector), nil
			case *extensions.Deployment:
				// TODO(madhusudancs): Make this smarter by admitting MatchExpressions with Equals
				// operator, DoubleEquals operator and In operator with only one element in the set.
				if len(t.Spec.Selector.MatchExpressions) > 0 {
					return "", fmt.Errorf("couldn't convert expressions - \"%+v\" to map-based selector format")
				}
				return kubectl.MakeLabels(t.Spec.Selector.MatchLabels), nil
			case *extensions.ReplicaSet:
				// TODO(madhusudancs): Make this smarter by admitting MatchExpressions with Equals
				// operator, DoubleEquals operator and In operator with only one element in the set.
				if len(t.Spec.Selector.MatchExpressions) > 0 {
					return "", fmt.Errorf("couldn't convert expressions - \"%+v\" to map-based selector format")
				}
				return kubectl.MakeLabels(t.Spec.Selector.MatchLabels), nil
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return "", err
				}
				return "", fmt.Errorf("cannot extract pod selector from %v", gvk)
			}
		},
		PortsForObject: func(object runtime.Object) ([]string, error) {
			// TODO: replace with a swagger schema based approach (identify pod selector via schema introspection)
			switch t := object.(type) {
			case *api.ReplicationController:
				return getPorts(t.Spec.Template.Spec), nil
			case *api.Pod:
				return getPorts(t.Spec), nil
			case *api.Service:
				return getServicePorts(t.Spec), nil
			case *extensions.Deployment:
				return getPorts(t.Spec.Template.Spec), nil
			case *extensions.ReplicaSet:
				return getPorts(t.Spec.Template.Spec), nil
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("cannot extract ports from %v", gvk)
			}
		},
		LabelsForObject: func(object runtime.Object) (map[string]string, error) {
			return meta.NewAccessor().Labels(object)
		},
		LogsForObject: func(object, options runtime.Object) (*client.Request, error) {
			c, err := clients.ClientForVersion(nil)
			if err != nil {
				return nil, err
			}

			switch t := object.(type) {
			case *api.Pod:
				opts, ok := options.(*api.PodLogOptions)
				if !ok {
					return nil, errors.New("provided options object is not a PodLogOptions")
				}
				return c.Pods(t.Namespace).GetLogs(t.Name, opts), nil
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("cannot get the logs from %v", gvk)
			}
		},
		PauseObject: func(object runtime.Object) (bool, error) {
			c, err := clients.ClientForVersion(nil)
			if err != nil {
				return false, err
			}

			switch t := object.(type) {
			case *extensions.Deployment:
				if t.Spec.Paused {
					return true, nil
				}
				t.Spec.Paused = true
				_, err := c.Extensions().Deployments(t.Namespace).Update(t)
				return false, err
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return false, err
				}
				return false, fmt.Errorf("cannot pause %v", gvk)
			}
		},
		ResumeObject: func(object runtime.Object) (bool, error) {
			c, err := clients.ClientForVersion(nil)
			if err != nil {
				return false, err
			}

			switch t := object.(type) {
			case *extensions.Deployment:
				if !t.Spec.Paused {
					return true, nil
				}
				t.Spec.Paused = false
				_, err := c.Extensions().Deployments(t.Namespace).Update(t)
				return false, err
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return false, err
				}
				return false, fmt.Errorf("cannot resume %v", gvk)
			}
		},
		Scaler: func(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			if err != nil {
				return nil, err
			}
			return kubectl.ScalerFor(mapping.GroupVersionKind.GroupKind(), client)
		},
		Reaper: func(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			if err != nil {
				return nil, err
			}
			return kubectl.ReaperFor(mapping.GroupVersionKind.GroupKind(), client)
		},
		HistoryViewer: func(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			clientset := clientset.FromUnversionedClient(client)
			if err != nil {
				return nil, err
			}
			return kubectl.HistoryViewerFor(mapping.GroupVersionKind.GroupKind(), clientset)
		},
		Rollbacker: func(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
			mappingVersion := mapping.GroupVersionKind.GroupVersion()
			client, err := clients.ClientForVersion(&mappingVersion)
			if err != nil {
				return nil, err
			}
			return kubectl.RollbackerFor(mapping.GroupVersionKind.GroupKind(), client)
		},
		Validator: func(validate bool, cacheDir string) (validation.Schema, error) {
			if validate {
				client, err := clients.ClientForVersion(nil)
				if err != nil {
					return nil, err
				}
				dir := cacheDir
				if len(dir) > 0 {
					version, err := client.ServerVersion()
					if err != nil {
						return nil, err
					}
					dir = path.Join(cacheDir, version.String())
				}
				return &clientSwaggerSchema{
					c:        client,
					cacheDir: dir,
					mapper:   api.RESTMapper,
				}, nil
			}
			return validation.NullSchema{}, nil
		},
		SwaggerSchema: func(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error) {
			version := gvk.GroupVersion()
			client, err := clients.ClientForVersion(&version)
			if err != nil {
				return nil, err
			}
			return client.Discovery().SwaggerSchema(version)
		},
		DefaultNamespace: func() (string, bool, error) {
			return clientConfig.Namespace()
		},
		Generators: func(cmdName string) map[string]kubectl.Generator {
			return DefaultGenerators(cmdName)
		},
		CanBeExposed: func(kind unversioned.GroupKind) error {
			switch kind {
			case api.Kind("ReplicationController"), api.Kind("Service"), api.Kind("Pod"), extensions.Kind("Deployment"), extensions.Kind("ReplicaSet"):
				// nothing to do here
			default:
				return fmt.Errorf("cannot expose a %s", kind)
			}
			return nil
		},
		CanBeAutoscaled: func(kind unversioned.GroupKind) error {
			switch kind {
			case api.Kind("ReplicationController"), extensions.Kind("Deployment"):
				// nothing to do here
			default:
				return fmt.Errorf("cannot autoscale a %v", kind)
			}
			return nil
		},
		AttachablePodForObject: func(object runtime.Object) (*api.Pod, error) {
			client, err := clients.ClientForVersion(nil)
			if err != nil {
				return nil, err
			}
			switch t := object.(type) {
			case *api.ReplicationController:
				selector := labels.SelectorFromSet(t.Spec.Selector)
				return GetFirstPod(client, t.Namespace, selector)
			case *extensions.Deployment:
				selector, err := unversioned.LabelSelectorAsSelector(t.Spec.Selector)
				if err != nil {
					return nil, fmt.Errorf("invalid label selector: %v", err)
				}
				return GetFirstPod(client, t.Namespace, selector)
			case *extensions.Job:
				selector, err := unversioned.LabelSelectorAsSelector(t.Spec.Selector)
				if err != nil {
					return nil, fmt.Errorf("invalid label selector: %v", err)
				}
				return GetFirstPod(client, t.Namespace, selector)
			case *api.Pod:
				return t, nil
			default:
				gvk, err := api.Scheme.ObjectKind(object)
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("cannot attach to %v: not implemented", gvk)
			}
		},
		EditorEnvs: func() []string {
			return []string{"KUBE_EDITOR", "EDITOR"}
		},
	}
}

// GetFirstPod returns the first pod of an object from its namespace and selector
func GetFirstPod(client *client.Client, namespace string, selector labels.Selector) (*api.Pod, error) {
	var pods *api.PodList
	for pods == nil || len(pods.Items) == 0 {
		var err error
		options := api.ListOptions{LabelSelector: selector}
		if pods, err = client.Pods(namespace).List(options); err != nil {
			return nil, err
		}
		if len(pods.Items) == 0 {
			time.Sleep(2 * time.Second)
		}
	}
	pod := &pods.Items[0]
	return pod, nil
}

func recordCommand(args []string) string {
	if len(args) > 0 {
		args[0] = "kubectl"
	}
	return strings.Join(args, " ")
}

func (f *Factory) Command() string {
	return f.cmd
}

// BindFlags adds any flags that are common to all kubectl sub commands.
func (f *Factory) BindFlags(flags *pflag.FlagSet) {
	// any flags defined by external projects (not part of pflags)
	flags.AddGoFlagSet(flag.CommandLine)

	// Merge factory's flags
	flags.AddFlagSet(f.flags)

	// Globally persistent flags across all subcommands.
	// TODO Change flag names to consts to allow safer lookup from subcommands.
	// TODO Add a verbose flag that turns on glog logging. Probably need a way
	// to do that automatically for every subcommand.
	flags.BoolVar(&f.clients.matchVersion, FlagMatchBinaryVersion, false, "Require server version to match client version")

	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. glog package
	flags.SetNormalizeFunc(util.WordSepNormalizeFunc)
}

func getPorts(spec api.PodSpec) []string {
	result := []string{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result = append(result, strconv.Itoa(port.ContainerPort))
		}
	}
	return result
}

// Extracts the ports exposed by a service from the given service spec.
func getServicePorts(spec api.ServiceSpec) []string {
	result := []string{}
	for _, servicePort := range spec.Ports {
		result = append(result, strconv.Itoa(servicePort.Port))
	}
	return result
}

type clientSwaggerSchema struct {
	c        *client.Client
	cacheDir string
	mapper   meta.RESTMapper
}

const schemaFileName = "schema.json"

type schemaClient interface {
	Get() *client.Request
}

func recursiveSplit(dir string) []string {
	parent, file := path.Split(dir)
	if len(parent) == 0 {
		return []string{file}
	}
	return append(recursiveSplit(parent[:len(parent)-1]), file)
}

func substituteUserHome(dir string) (string, error) {
	if len(dir) == 0 || dir[0] != '~' {
		return dir, nil
	}
	parts := recursiveSplit(dir)
	if len(parts[0]) == 1 {
		parts[0] = os.Getenv("HOME")
	} else {
		usr, err := user.Lookup(parts[0][1:])
		if err != nil {
			return "", err
		}
		parts[0] = usr.HomeDir
	}
	return path.Join(parts...), nil
}

func writeSchemaFile(schemaData []byte, cacheDir, cacheFile, prefix, groupVersion string) error {
	if err := os.MkdirAll(path.Join(cacheDir, prefix, groupVersion), 0755); err != nil {
		return err
	}
	tmpFile, err := ioutil.TempFile(cacheDir, "schema")
	if err != nil {
		// If we can't write, keep going.
		if os.IsPermission(err) {
			return nil
		}
		return err
	}
	if _, err := io.Copy(tmpFile, bytes.NewBuffer(schemaData)); err != nil {
		return err
	}
	if err := os.Link(tmpFile.Name(), cacheFile); err != nil {
		// If we can't write due to file existing, or permission problems, keep going.
		if os.IsExist(err) || os.IsPermission(err) {
			return nil
		}
		return err
	}
	return nil
}

func getSchemaAndValidate(c schemaClient, data []byte, prefix, groupVersion, cacheDir string) (err error) {
	var schemaData []byte
	fullDir, err := substituteUserHome(cacheDir)
	if err != nil {
		return err
	}
	cacheFile := path.Join(fullDir, prefix, groupVersion, schemaFileName)

	if len(cacheDir) != 0 {
		if schemaData, err = ioutil.ReadFile(cacheFile); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if schemaData == nil {
		schemaData, err = c.Get().
			AbsPath("/swaggerapi", prefix, groupVersion).
			Do().
			Raw()
		if err != nil {
			return err
		}
		if len(cacheDir) != 0 {
			if err := writeSchemaFile(schemaData, fullDir, cacheFile, prefix, groupVersion); err != nil {
				return err
			}
		}
	}
	schema, err := validation.NewSwaggerSchemaFromBytes(schemaData)
	if err != nil {
		return err
	}
	return schema.ValidateBytes(data)
}

func (c *clientSwaggerSchema) ValidateBytes(data []byte) error {
	gvk, err := json.DefaultMetaFactory.Interpret(data)
	if err != nil {
		return err
	}
	if ok := registered.IsEnabledVersion(gvk.GroupVersion()); !ok {
		return fmt.Errorf("API version %q isn't supported, only supports API versions %q", gvk.GroupVersion().String(), registered.EnabledVersions())
	}
	if gvk.Group == autoscaling.GroupName {
		if c.c.AutoscalingClient == nil {
			return errors.New("unable to validate: no autoscaling client")
		}
		return getSchemaAndValidate(c.c.AutoscalingClient.RESTClient, data, "apis/", gvk.GroupVersion().String(), c.cacheDir)
	}
	if gvk.Group == extensions.GroupName {
		if c.c.ExtensionsClient == nil {
			return errors.New("unable to validate: no experimental client")
		}
		return getSchemaAndValidate(c.c.ExtensionsClient.RESTClient, data, "apis/", gvk.GroupVersion().String(), c.cacheDir)
	}
	return getSchemaAndValidate(c.c.RESTClient, data, "api", gvk.GroupVersion().String(), c.cacheDir)
}

// DefaultClientConfig creates a clientcmd.ClientConfig with the following hierarchy:
//   1.  Use the kubeconfig builder.  The number of merges and overrides here gets a little crazy.  Stay with me.
//       1.  Merge together the kubeconfig itself.  This is done with the following hierarchy rules:
//           1.  CommandLineLocation - this parsed from the command line, so it must be late bound.  If you specify this,
//               then no other kubeconfig files are merged.  This file must exist.
//           2.  If $KUBECONFIG is set, then it is treated as a list of files that should be merged.
//	     3.  HomeDirectoryLocation
//           Empty filenames are ignored.  Files with non-deserializable content produced errors.
//           The first file to set a particular value or map key wins and the value or map key is never changed.
//           This means that the first file to set CurrentContext will have its context preserved.  It also means
//           that if two files specify a "red-user", only values from the first file's red-user are used.  Even
//           non-conflicting entries from the second file's "red-user" are discarded.
//       2.  Determine the context to use based on the first hit in this chain
//           1.  command line argument - again, parsed from the command line, so it must be late bound
//           2.  CurrentContext from the merged kubeconfig file
//           3.  Empty is allowed at this stage
//       3.  Determine the cluster info and auth info to use.  At this point, we may or may not have a context.  They
//           are built based on the first hit in this chain.  (run it twice, once for auth, once for cluster)
//           1.  command line argument
//           2.  If context is present, then use the context value
//           3.  Empty is allowed
//       4.  Determine the actual cluster info to use.  At this point, we may or may not have a cluster info.  Build
//           each piece of the cluster info based on the chain:
//           1.  command line argument
//           2.  If cluster info is present and a value for the attribute is present, use it.
//           3.  If you don't have a server location, bail.
//       5.  Auth info is build using the same rules as cluster info, EXCEPT that you can only have one authentication
//           technique per auth info.  The following conditions result in an error:
//           1.  If there are two conflicting techniques specified from the command line, fail.
//           2.  If the command line does not specify one, and the auth info has conflicting techniques, fail.
//           3.  If the command line specifies one and the auth info specifies another, honor the command line technique.
//   2.  Use default values and potentially prompt for auth information
//
//   However, if it appears that we're running in a kubernetes cluster
//   container environment, then run with the auth info kubernetes mounted for
//   us. Specifically:
//     The env vars KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT are
//     set, and the file /var/run/secrets/kubernetes.io/serviceaccount/token
//     exists and is not a directory.
func DefaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	flags.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	flagNames := clientcmd.RecommendedConfigOverrideFlags("")
	// short flagnames are disabled by default.  These are here for compatibility with existing scripts
	flagNames.ClusterOverrideFlags.APIServer.ShortName = "s"

	clientcmd.BindOverrideFlags(overrides, flags, flagNames)
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin)

	return clientConfig
}

// PrintObject prints an api object given command line flags to modify the output format
func (f *Factory) PrintObject(cmd *cobra.Command, obj runtime.Object, out io.Writer) error {
	mapper, _ := f.Object()
	gvk, err := api.Scheme.ObjectKind(obj)
	if err != nil {
		return err
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind())
	if err != nil {
		return err
	}

	printer, err := f.PrinterForMapping(cmd, mapping, false)
	if err != nil {
		return err
	}
	return printer.PrintObj(obj, out)
}

// PrinterForMapping returns a printer suitable for displaying the provided resource type.
// Requires that printer flags have been added to cmd (see AddPrinterFlags).
func (f *Factory) PrinterForMapping(cmd *cobra.Command, mapping *meta.RESTMapping, withNamespace bool) (kubectl.ResourcePrinter, error) {
	printer, ok, err := PrinterForCommand(cmd)
	if err != nil {
		return nil, err
	}
	if ok {
		clientConfig, err := f.ClientConfig()
		if err != nil {
			return nil, err
		}

		version, err := OutputVersion(cmd, clientConfig.GroupVersion)
		if err != nil {
			return nil, err
		}
		if version.IsEmpty() {
			version = mapping.GroupVersionKind.GroupVersion()
		}
		if version.IsEmpty() {
			return nil, fmt.Errorf("you must specify an output-version when using this output format")
		}

		printer = kubectl.NewVersionedPrinter(printer, mapping.ObjectConvertor, version, mapping.GroupVersionKind.GroupVersion())

	} else {
		// Some callers do not have "label-columns" so we can't use the GetFlagStringSlice() helper
		columnLabel, err := cmd.Flags().GetStringSlice("label-columns")
		if err != nil {
			columnLabel = []string{}
		}
		printer, err = f.Printer(mapping, GetFlagBool(cmd, "no-headers"), withNamespace, GetWideFlag(cmd), GetFlagBool(cmd, "show-all"), GetFlagBool(cmd, "show-labels"), isWatch(cmd), columnLabel)
		if err != nil {
			return nil, err
		}
		printer = maybeWrapSortingPrinter(cmd, printer)
	}

	return printer, nil
}

// One stop shopping for a Builder
func (f *Factory) NewBuilder() *resource.Builder {
	mapper, typer := f.Object()

	return resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true))
}
