/*
Copyright 2015 The Kubernetes Authors.

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

package v1

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

const (
	// Annotation key used to identify mirror pods.
	mirrorAnnotationKey = "kubernetes.io/config.mirror"

	// Value used to identify mirror pods from pre-v1.1 kubelet.
	mirrorAnnotationValue_1_0 = "mirror"

	// annotation key prefix used to identify non-convertible json paths.
	NonConvertibleAnnotationPrefix = "kubernetes.io/non-convertible"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	// Add non-generated conversion functions
	err := scheme.AddConversionFuncs(
		Convert_api_Pod_To_v1_Pod,
		Convert_api_PodSpec_To_v1_PodSpec,
		Convert_api_ReplicationControllerSpec_To_v1_ReplicationControllerSpec,
		Convert_api_ServiceSpec_To_v1_ServiceSpec,
		Convert_v1_Pod_To_api_Pod,
		Convert_v1_PodSpec_To_api_PodSpec,
		Convert_v1_ReplicationControllerSpec_To_api_ReplicationControllerSpec,
		Convert_v1_Secret_To_api_Secret,
		Convert_v1_ServiceSpec_To_api_ServiceSpec,
		Convert_v1_ResourceList_To_api_ResourceList,
		Convert_v1_ReplicationController_to_extensions_ReplicaSet,
		Convert_v1_ReplicationControllerSpec_to_extensions_ReplicaSetSpec,
		Convert_v1_ReplicationControllerStatus_to_extensions_ReplicaSetStatus,
		Convert_extensions_ReplicaSet_to_v1_ReplicationController,
		Convert_extensions_ReplicaSetSpec_to_v1_ReplicationControllerSpec,
		Convert_extensions_ReplicaSetStatus_to_v1_ReplicationControllerStatus,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	// Add field label conversions for kinds having selectable nothing but ObjectMeta fields.
	for _, kind := range []string{
		"Endpoints",
		"ResourceQuota",
		"PersistentVolumeClaim",
		"Service",
		"ServiceAccount",
		"ConfigMap",
	} {
		err = api.Scheme.AddFieldLabelConversionFunc("v1", kind,
			func(label, value string) (string, string, error) {
				switch label {
				case "metadata.namespace",
					"metadata.name":
					return label, value, nil
				default:
					return "", "", fmt.Errorf("field label %q not supported for %q", label, kind)
				}
			})
		if err != nil {
			// If one of the conversion functions is malformed, detect it immediately.
			panic(err)
		}
	}

	// Add field conversion funcs.
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Pod",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"metadata.labels",
				"metadata.annotations",
				"status.phase",
				"status.podIP",
				"spec.nodeName",
				"spec.restartPolicy":
				return label, value, nil
				// This is for backwards compatibility with old v1 clients which send spec.host
			case "spec.host":
				return "spec.nodeName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Node",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			case "spec.unschedulable":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "ReplicationController",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"status.replicas":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Event",
		func(label, value string) (string, string, error) {
			switch label {
			case "involvedObject.kind",
				"involvedObject.namespace",
				"involvedObject.name",
				"involvedObject.uid",
				"involvedObject.apiVersion",
				"involvedObject.resourceVersion",
				"involvedObject.fieldPath",
				"reason",
				"source",
				"type",
				"metadata.namespace",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Namespace",
		func(label, value string) (string, string, error) {
			switch label {
			case "status.phase",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "PersistentVolume",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Secret",
		func(label, value string) (string, string, error) {
			switch label {
			case "type",
				"metadata.namespace",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func Convert_v1_ReplicationController_to_extensions_ReplicaSet(in *ReplicationController, out *extensions.ReplicaSet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationController))(in)
	}
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_ReplicationControllerSpec_to_extensions_ReplicaSetSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_ReplicationControllerStatus_to_extensions_ReplicaSetStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ReplicationControllerSpec_to_extensions_ReplicaSetSpec(in *ReplicationControllerSpec, out *extensions.ReplicaSetSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationControllerSpec))(in)
	}
	out.Replicas = *in.Replicas
	if in.Selector != nil {
		api.Convert_map_to_unversioned_LabelSelector(&in.Selector, out.Selector, s)
	}
	if in.Template != nil {
		if err := Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, &out.Template, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1_ReplicationControllerStatus_to_extensions_ReplicaSetStatus(in *ReplicationControllerStatus, out *extensions.ReplicaSetStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationControllerStatus))(in)
	}
	out.Replicas = in.Replicas
	out.FullyLabeledReplicas = in.FullyLabeledReplicas
	out.ObservedGeneration = in.ObservedGeneration
	return nil
}

func Convert_extensions_ReplicaSet_to_v1_ReplicationController(in *extensions.ReplicaSet, out *ReplicationController, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*extensions.ReplicaSet))(in)
	}
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_extensions_ReplicaSetSpec_to_v1_ReplicationControllerSpec(&in.Spec, &out.Spec, s); err != nil {
		fieldErr, ok := err.(*field.Error)
		if !ok {
			return err
		}
		if out.Annotations == nil {
			out.Annotations = make(map[string]string)
		}
		out.Annotations[NonConvertibleAnnotationPrefix+"/"+fieldErr.Field] = reflect.ValueOf(fieldErr.BadValue).String()
	}
	if err := Convert_extensions_ReplicaSetStatus_to_v1_ReplicationControllerStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_ReplicaSetSpec_to_v1_ReplicationControllerSpec(in *extensions.ReplicaSetSpec, out *ReplicationControllerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*extensions.ReplicaSetSpec))(in)
	}
	out.Replicas = new(int32)
	*out.Replicas = in.Replicas
	var invalidErr error
	if in.Selector != nil {
		invalidErr = api.Convert_unversioned_LabelSelector_to_map(in.Selector, &out.Selector, s)
	}
	out.Template = new(PodTemplateSpec)
	if err := Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, out.Template, s); err != nil {
		return err
	}
	return invalidErr
}

func Convert_extensions_ReplicaSetStatus_to_v1_ReplicationControllerStatus(in *extensions.ReplicaSetStatus, out *ReplicationControllerStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*extensions.ReplicaSetStatus))(in)
	}
	out.Replicas = in.Replicas
	out.FullyLabeledReplicas = in.FullyLabeledReplicas
	out.ObservedGeneration = in.ObservedGeneration
	return nil
}

func Convert_api_ReplicationControllerSpec_To_v1_ReplicationControllerSpec(in *api.ReplicationControllerSpec, out *ReplicationControllerSpec, s conversion.Scope) error {
	out.Replicas = &in.Replicas
	out.Selector = in.Selector
	if in.Template != nil {
		out.Template = new(PodTemplateSpec)
		if err := Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func Convert_v1_ReplicationControllerSpec_To_api_ReplicationControllerSpec(in *ReplicationControllerSpec, out *api.ReplicationControllerSpec, s conversion.Scope) error {
	out.Replicas = *in.Replicas
	out.Selector = in.Selector
	if in.Template != nil {
		out.Template = new(api.PodTemplateSpec)
		if err := Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func Convert_api_PodStatusResult_To_v1_PodStatusResult(in *api.PodStatusResult, out *PodStatusResult, s conversion.Scope) error {
	if err := autoConvert_api_PodStatusResult_To_v1_PodStatusResult(in, out, s); err != nil {
		return err
	}

	if old := out.Annotations; old != nil {
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
	}
	if len(out.Status.InitContainerStatuses) > 0 {
		if out.Annotations == nil {
			out.Annotations = make(map[string]string)
		}
		value, err := json.Marshal(out.Status.InitContainerStatuses)
		if err != nil {
			return err
		}
		out.Annotations[PodInitContainerStatusesAnnotationKey] = string(value)
	} else {
		delete(out.Annotations, PodInitContainerStatusesAnnotationKey)
	}
	return nil
}

func Convert_v1_PodStatusResult_To_api_PodStatusResult(in *PodStatusResult, out *api.PodStatusResult, s conversion.Scope) error {
	// TODO: when we move init container to beta, remove these conversions
	if value, ok := in.Annotations[PodInitContainerStatusesAnnotationKey]; ok {
		var values []ContainerStatus
		if err := json.Unmarshal([]byte(value), &values); err != nil {
			return err
		}
		// Conversion from external to internal version exists more to
		// satisfy the needs of the decoder than it does to be a general
		// purpose tool. And Decode always creates an intermediate object
		// to decode to. Thus the caller of UnsafeConvertToVersion is
		// taking responsibility to ensure mutation of in is not exposed
		// back to the caller.
		in.Status.InitContainerStatuses = values
	}

	if err := autoConvert_v1_PodStatusResult_To_api_PodStatusResult(in, out, s); err != nil {
		return err
	}
	if len(out.Annotations) > 0 {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
		delete(out.Annotations, PodInitContainerStatusesAnnotationKey)
	}
	return nil
}

func Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in *api.PodTemplateSpec, out *PodTemplateSpec, s conversion.Scope) error {
	if err := autoConvert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in, out, s); err != nil {
		return err
	}

	// TODO: when we move init container to beta, remove these conversions
	if old := out.Annotations; old != nil {
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
	}
	if len(out.Spec.InitContainers) > 0 {
		if out.Annotations == nil {
			out.Annotations = make(map[string]string)
		}
		value, err := json.Marshal(out.Spec.InitContainers)
		if err != nil {
			return err
		}
		out.Annotations[PodInitContainersAnnotationKey] = string(value)
	} else {
		delete(out.Annotations, PodInitContainersAnnotationKey)
	}
	return nil
}

func Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in *PodTemplateSpec, out *api.PodTemplateSpec, s conversion.Scope) error {
	// TODO: when we move init container to beta, remove these conversions
	if value, ok := in.Annotations[PodInitContainersAnnotationKey]; ok {
		var values []Container
		if err := json.Unmarshal([]byte(value), &values); err != nil {
			return err
		}
		// Conversion from external to internal version exists more to
		// satisfy the needs of the decoder than it does to be a general
		// purpose tool. And Decode always creates an intermediate object
		// to decode to. Thus the caller of UnsafeConvertToVersion is
		// taking responsibility to ensure mutation of in is not exposed
		// back to the caller.
		in.Spec.InitContainers = values
	}

	if err := autoConvert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in, out, s); err != nil {
		return err
	}
	if len(out.Annotations) > 0 {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
		delete(out.Annotations, PodInitContainersAnnotationKey)
	}
	return nil
}

// The following two PodSpec conversions are done here to support ServiceAccount
// as an alias for ServiceAccountName.
func Convert_api_PodSpec_To_v1_PodSpec(in *api.PodSpec, out *PodSpec, s conversion.Scope) error {
	if err := autoConvert_api_PodSpec_To_v1_PodSpec(in, out, s); err != nil {
		return err
	}

	// DeprecatedServiceAccount is an alias for ServiceAccountName.
	out.DeprecatedServiceAccount = in.ServiceAccountName

	if in.SecurityContext != nil {
		// the host namespace fields have to be handled here for backward compatibility
		// with v1.0.0
		out.HostPID = in.SecurityContext.HostPID
		out.HostNetwork = in.SecurityContext.HostNetwork
		out.HostIPC = in.SecurityContext.HostIPC
	}

	return nil
}

func Convert_v1_PodSpec_To_api_PodSpec(in *PodSpec, out *api.PodSpec, s conversion.Scope) error {
	if err := autoConvert_v1_PodSpec_To_api_PodSpec(in, out, s); err != nil {
		return err
	}

	// We support DeprecatedServiceAccount as an alias for ServiceAccountName.
	// If both are specified, ServiceAccountName (the new field) wins.
	if in.ServiceAccountName == "" {
		out.ServiceAccountName = in.DeprecatedServiceAccount
	}

	// the host namespace fields have to be handled specially for backward compatibility
	// with v1.0.0
	if out.SecurityContext == nil {
		out.SecurityContext = new(api.PodSecurityContext)
	}
	out.SecurityContext.HostNetwork = in.HostNetwork
	out.SecurityContext.HostPID = in.HostPID
	out.SecurityContext.HostIPC = in.HostIPC

	return nil
}

func Convert_api_Pod_To_v1_Pod(in *api.Pod, out *Pod, s conversion.Scope) error {
	if err := autoConvert_api_Pod_To_v1_Pod(in, out, s); err != nil {
		return err
	}

	// TODO: when we move init container to beta, remove these conversions
	if len(out.Spec.InitContainers) > 0 || len(out.Status.InitContainerStatuses) > 0 {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
		delete(out.Annotations, PodInitContainersAnnotationKey)
		delete(out.Annotations, PodInitContainerStatusesAnnotationKey)
	}
	if len(out.Spec.InitContainers) > 0 {
		value, err := json.Marshal(out.Spec.InitContainers)
		if err != nil {
			return err
		}
		out.Annotations[PodInitContainersAnnotationKey] = string(value)
	}
	if len(out.Status.InitContainerStatuses) > 0 {
		value, err := json.Marshal(out.Status.InitContainerStatuses)
		if err != nil {
			return err
		}
		out.Annotations[PodInitContainerStatusesAnnotationKey] = string(value)
	}

	// We need to reset certain fields for mirror pods from pre-v1.1 kubelet
	// (#15960).
	// TODO: Remove this code after we drop support for v1.0 kubelets.
	if value, ok := in.Annotations[mirrorAnnotationKey]; ok && value == mirrorAnnotationValue_1_0 {
		// Reset the TerminationGracePeriodSeconds.
		out.Spec.TerminationGracePeriodSeconds = nil
		// Reset the resource requests.
		for i := range out.Spec.Containers {
			out.Spec.Containers[i].Resources.Requests = nil
		}
	}
	return nil
}

func Convert_v1_Pod_To_api_Pod(in *Pod, out *api.Pod, s conversion.Scope) error {
	// TODO: when we move init container to beta, remove these conversions
	if value, ok := in.Annotations[PodInitContainersAnnotationKey]; ok {
		var values []Container
		if err := json.Unmarshal([]byte(value), &values); err != nil {
			return err
		}
		// Conversion from external to internal version exists more to
		// satisfy the needs of the decoder than it does to be a general
		// purpose tool. And Decode always creates an intermediate object
		// to decode to. Thus the caller of UnsafeConvertToVersion is
		// taking responsibility to ensure mutation of in is not exposed
		// back to the caller.
		in.Spec.InitContainers = values
	}
	if value, ok := in.Annotations[PodInitContainerStatusesAnnotationKey]; ok {
		var values []ContainerStatus
		if err := json.Unmarshal([]byte(value), &values); err != nil {
			return err
		}
		// Conversion from external to internal version exists more to
		// satisfy the needs of the decoder than it does to be a general
		// purpose tool. And Decode always creates an intermediate object
		// to decode to. Thus the caller of UnsafeConvertToVersion is
		// taking responsibility to ensure mutation of in is not exposed
		// back to the caller.
		in.Status.InitContainerStatuses = values
	}

	if err := autoConvert_v1_Pod_To_api_Pod(in, out, s); err != nil {
		return err
	}
	if len(out.Annotations) > 0 {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old))
		for k, v := range old {
			out.Annotations[k] = v
		}
		delete(out.Annotations, PodInitContainersAnnotationKey)
		delete(out.Annotations, PodInitContainerStatusesAnnotationKey)
	}
	return nil
}

func Convert_api_ServiceSpec_To_v1_ServiceSpec(in *api.ServiceSpec, out *ServiceSpec, s conversion.Scope) error {
	if err := autoConvert_api_ServiceSpec_To_v1_ServiceSpec(in, out, s); err != nil {
		return err
	}
	// Publish both externalIPs and deprecatedPublicIPs fields in v1.
	out.DeprecatedPublicIPs = in.ExternalIPs
	return nil
}

func Convert_v1_Secret_To_api_Secret(in *Secret, out *api.Secret, s conversion.Scope) error {
	if err := autoConvert_v1_Secret_To_api_Secret(in, out, s); err != nil {
		return err
	}

	// StringData overwrites Data
	if len(in.StringData) > 0 {
		if out.Data == nil {
			out.Data = map[string][]byte{}
		}
		for k, v := range in.StringData {
			out.Data[k] = []byte(v)
		}
	}

	return nil
}

func Convert_v1_ServiceSpec_To_api_ServiceSpec(in *ServiceSpec, out *api.ServiceSpec, s conversion.Scope) error {
	if err := autoConvert_v1_ServiceSpec_To_api_ServiceSpec(in, out, s); err != nil {
		return err
	}
	// Prefer the legacy deprecatedPublicIPs field, if provided.
	if len(in.DeprecatedPublicIPs) > 0 {
		out.ExternalIPs = in.DeprecatedPublicIPs
	}
	return nil
}

func Convert_api_PodSecurityContext_To_v1_PodSecurityContext(in *api.PodSecurityContext, out *PodSecurityContext, s conversion.Scope) error {
	out.SupplementalGroups = in.SupplementalGroups
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(SELinuxOptions)
		if err := Convert_api_SELinuxOptions_To_v1_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
			return err
		}
	} else {
		out.SELinuxOptions = nil
	}
	out.RunAsUser = in.RunAsUser
	out.RunAsNonRoot = in.RunAsNonRoot
	out.FSGroup = in.FSGroup
	return nil
}

func Convert_v1_PodSecurityContext_To_api_PodSecurityContext(in *PodSecurityContext, out *api.PodSecurityContext, s conversion.Scope) error {
	out.SupplementalGroups = in.SupplementalGroups
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(api.SELinuxOptions)
		if err := Convert_v1_SELinuxOptions_To_api_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
			return err
		}
	} else {
		out.SELinuxOptions = nil
	}
	out.RunAsUser = in.RunAsUser
	out.RunAsNonRoot = in.RunAsNonRoot
	out.FSGroup = in.FSGroup
	return nil
}

func Convert_v1_ResourceList_To_api_ResourceList(in *ResourceList, out *api.ResourceList, s conversion.Scope) error {
	if *in == nil {
		return nil
	}

	if *out == nil {
		*out = make(api.ResourceList, len(*in))
	}
	for key, val := range *in {
		// TODO(#18538): We round up resource values to milli scale to maintain API compatibility.
		// In the future, we should instead reject values that need rounding.
		const milliScale = -3
		val.RoundUp(milliScale)

		(*out)[api.ResourceName(key)] = val
	}
	return nil
}
