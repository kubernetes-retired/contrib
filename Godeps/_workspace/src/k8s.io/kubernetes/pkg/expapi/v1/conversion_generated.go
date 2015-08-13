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

package v1

// AUTO-GENERATED FUNCTIONS START HERE
import (
	reflect "reflect"

	api "k8s.io/kubernetes/pkg/api"
	v1 "k8s.io/kubernetes/pkg/api/v1"
	conversion "k8s.io/kubernetes/pkg/conversion"
	expapi "k8s.io/kubernetes/pkg/expapi"
)

func convert_api_ListMeta_To_v1_ListMeta(in *api.ListMeta, out *v1.ListMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ListMeta))(in)
	}
	out.SelfLink = in.SelfLink
	out.ResourceVersion = in.ResourceVersion
	return nil
}

func convert_api_ObjectMeta_To_v1_ObjectMeta(in *api.ObjectMeta, out *v1.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := s.Convert(&in.CreationTimestamp, &out.CreationTimestamp, 0); err != nil {
		return err
	}
	if in.DeletionTimestamp != nil {
		if err := s.Convert(&in.DeletionTimestamp, &out.DeletionTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.DeletionTimestamp = nil
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	} else {
		out.Labels = nil
	}
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	return nil
}

func convert_api_TypeMeta_To_v1_TypeMeta(in *api.TypeMeta, out *v1.TypeMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.TypeMeta))(in)
	}
	out.Kind = in.Kind
	out.APIVersion = in.APIVersion
	return nil
}

func convert_v1_ListMeta_To_api_ListMeta(in *v1.ListMeta, out *api.ListMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ListMeta))(in)
	}
	out.SelfLink = in.SelfLink
	out.ResourceVersion = in.ResourceVersion
	return nil
}

func convert_v1_ObjectMeta_To_api_ObjectMeta(in *v1.ObjectMeta, out *api.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := s.Convert(&in.CreationTimestamp, &out.CreationTimestamp, 0); err != nil {
		return err
	}
	if in.DeletionTimestamp != nil {
		if err := s.Convert(&in.DeletionTimestamp, &out.DeletionTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.DeletionTimestamp = nil
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	} else {
		out.Labels = nil
	}
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	return nil
}

func convert_v1_TypeMeta_To_api_TypeMeta(in *v1.TypeMeta, out *api.TypeMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.TypeMeta))(in)
	}
	out.Kind = in.Kind
	out.APIVersion = in.APIVersion
	return nil
}

func convert_expapi_HorizontalPodAutoscaler_To_v1_HorizontalPodAutoscaler(in *expapi.HorizontalPodAutoscaler, out *HorizontalPodAutoscaler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.HorizontalPodAutoscaler))(in)
	}
	if err := convert_api_TypeMeta_To_v1_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_expapi_HorizontalPodAutoscalerSpec_To_v1_HorizontalPodAutoscalerSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_expapi_HorizontalPodAutoscalerList_To_v1_HorizontalPodAutoscalerList(in *expapi.HorizontalPodAutoscalerList, out *HorizontalPodAutoscalerList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.HorizontalPodAutoscalerList))(in)
	}
	if err := convert_api_TypeMeta_To_v1_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_api_ListMeta_To_v1_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]HorizontalPodAutoscaler, len(in.Items))
		for i := range in.Items {
			if err := convert_expapi_HorizontalPodAutoscaler_To_v1_HorizontalPodAutoscaler(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_expapi_HorizontalPodAutoscalerSpec_To_v1_HorizontalPodAutoscalerSpec(in *expapi.HorizontalPodAutoscalerSpec, out *HorizontalPodAutoscalerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.HorizontalPodAutoscalerSpec))(in)
	}
	if in.ScaleRef != nil {
		out.ScaleRef = new(SubresourceReference)
		if err := convert_expapi_SubresourceReference_To_v1_SubresourceReference(in.ScaleRef, out.ScaleRef, s); err != nil {
			return err
		}
	} else {
		out.ScaleRef = nil
	}
	out.MinCount = in.MinCount
	out.MaxCount = in.MaxCount
	if err := convert_expapi_TargetConsumption_To_v1_TargetConsumption(&in.Target, &out.Target, s); err != nil {
		return err
	}
	return nil
}

func convert_expapi_ReplicationControllerDummy_To_v1_ReplicationControllerDummy(in *expapi.ReplicationControllerDummy, out *ReplicationControllerDummy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.ReplicationControllerDummy))(in)
	}
	if err := convert_api_TypeMeta_To_v1_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	return nil
}

func convert_expapi_Scale_To_v1_Scale(in *expapi.Scale, out *Scale, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.Scale))(in)
	}
	if err := convert_api_TypeMeta_To_v1_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_expapi_ScaleSpec_To_v1_ScaleSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_expapi_ScaleStatus_To_v1_ScaleStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_expapi_ScaleSpec_To_v1_ScaleSpec(in *expapi.ScaleSpec, out *ScaleSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.ScaleSpec))(in)
	}
	out.Replicas = in.Replicas
	return nil
}

func convert_expapi_ScaleStatus_To_v1_ScaleStatus(in *expapi.ScaleStatus, out *ScaleStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.ScaleStatus))(in)
	}
	out.Replicas = in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	return nil
}

func convert_expapi_SubresourceReference_To_v1_SubresourceReference(in *expapi.SubresourceReference, out *SubresourceReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.SubresourceReference))(in)
	}
	out.Kind = in.Kind
	out.Namespace = in.Namespace
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	out.Subresource = in.Subresource
	return nil
}

func convert_expapi_TargetConsumption_To_v1_TargetConsumption(in *expapi.TargetConsumption, out *TargetConsumption, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*expapi.TargetConsumption))(in)
	}
	out.Resource = v1.ResourceName(in.Resource)
	if err := s.Convert(&in.Quantity, &out.Quantity, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_HorizontalPodAutoscaler_To_expapi_HorizontalPodAutoscaler(in *HorizontalPodAutoscaler, out *expapi.HorizontalPodAutoscaler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*HorizontalPodAutoscaler))(in)
	}
	if err := convert_v1_TypeMeta_To_api_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_HorizontalPodAutoscalerSpec_To_expapi_HorizontalPodAutoscalerSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_HorizontalPodAutoscalerList_To_expapi_HorizontalPodAutoscalerList(in *HorizontalPodAutoscalerList, out *expapi.HorizontalPodAutoscalerList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*HorizontalPodAutoscalerList))(in)
	}
	if err := convert_v1_TypeMeta_To_api_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ListMeta_To_api_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]expapi.HorizontalPodAutoscaler, len(in.Items))
		for i := range in.Items {
			if err := convert_v1_HorizontalPodAutoscaler_To_expapi_HorizontalPodAutoscaler(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_HorizontalPodAutoscalerSpec_To_expapi_HorizontalPodAutoscalerSpec(in *HorizontalPodAutoscalerSpec, out *expapi.HorizontalPodAutoscalerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*HorizontalPodAutoscalerSpec))(in)
	}
	if in.ScaleRef != nil {
		out.ScaleRef = new(expapi.SubresourceReference)
		if err := convert_v1_SubresourceReference_To_expapi_SubresourceReference(in.ScaleRef, out.ScaleRef, s); err != nil {
			return err
		}
	} else {
		out.ScaleRef = nil
	}
	out.MinCount = in.MinCount
	out.MaxCount = in.MaxCount
	if err := convert_v1_TargetConsumption_To_expapi_TargetConsumption(&in.Target, &out.Target, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_ReplicationControllerDummy_To_expapi_ReplicationControllerDummy(in *ReplicationControllerDummy, out *expapi.ReplicationControllerDummy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationControllerDummy))(in)
	}
	if err := convert_v1_TypeMeta_To_api_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_Scale_To_expapi_Scale(in *Scale, out *expapi.Scale, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*Scale))(in)
	}
	if err := convert_v1_TypeMeta_To_api_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ScaleSpec_To_expapi_ScaleSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_ScaleStatus_To_expapi_ScaleStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_ScaleSpec_To_expapi_ScaleSpec(in *ScaleSpec, out *expapi.ScaleSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ScaleSpec))(in)
	}
	out.Replicas = in.Replicas
	return nil
}

func convert_v1_ScaleStatus_To_expapi_ScaleStatus(in *ScaleStatus, out *expapi.ScaleStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ScaleStatus))(in)
	}
	out.Replicas = in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	return nil
}

func convert_v1_SubresourceReference_To_expapi_SubresourceReference(in *SubresourceReference, out *expapi.SubresourceReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*SubresourceReference))(in)
	}
	out.Kind = in.Kind
	out.Namespace = in.Namespace
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	out.Subresource = in.Subresource
	return nil
}

func convert_v1_TargetConsumption_To_expapi_TargetConsumption(in *TargetConsumption, out *expapi.TargetConsumption, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*TargetConsumption))(in)
	}
	out.Resource = api.ResourceName(in.Resource)
	if err := s.Convert(&in.Quantity, &out.Quantity, 0); err != nil {
		return err
	}
	return nil
}

func init() {
	err := api.Scheme.AddGeneratedConversionFuncs(
		convert_api_ListMeta_To_v1_ListMeta,
		convert_api_ObjectMeta_To_v1_ObjectMeta,
		convert_api_TypeMeta_To_v1_TypeMeta,
		convert_expapi_HorizontalPodAutoscalerList_To_v1_HorizontalPodAutoscalerList,
		convert_expapi_HorizontalPodAutoscalerSpec_To_v1_HorizontalPodAutoscalerSpec,
		convert_expapi_HorizontalPodAutoscaler_To_v1_HorizontalPodAutoscaler,
		convert_expapi_ReplicationControllerDummy_To_v1_ReplicationControllerDummy,
		convert_expapi_ScaleSpec_To_v1_ScaleSpec,
		convert_expapi_ScaleStatus_To_v1_ScaleStatus,
		convert_expapi_Scale_To_v1_Scale,
		convert_expapi_SubresourceReference_To_v1_SubresourceReference,
		convert_expapi_TargetConsumption_To_v1_TargetConsumption,
		convert_v1_HorizontalPodAutoscalerList_To_expapi_HorizontalPodAutoscalerList,
		convert_v1_HorizontalPodAutoscalerSpec_To_expapi_HorizontalPodAutoscalerSpec,
		convert_v1_HorizontalPodAutoscaler_To_expapi_HorizontalPodAutoscaler,
		convert_v1_ListMeta_To_api_ListMeta,
		convert_v1_ObjectMeta_To_api_ObjectMeta,
		convert_v1_ReplicationControllerDummy_To_expapi_ReplicationControllerDummy,
		convert_v1_ScaleSpec_To_expapi_ScaleSpec,
		convert_v1_ScaleStatus_To_expapi_ScaleStatus,
		convert_v1_Scale_To_expapi_Scale,
		convert_v1_SubresourceReference_To_expapi_SubresourceReference,
		convert_v1_TargetConsumption_To_expapi_TargetConsumption,
		convert_v1_TypeMeta_To_api_TypeMeta,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
