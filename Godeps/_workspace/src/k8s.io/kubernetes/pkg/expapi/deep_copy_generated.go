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

package expapi

// AUTO-GENERATED FUNCTIONS START HERE
import (
	time "time"

	api "k8s.io/kubernetes/pkg/api"
	resource "k8s.io/kubernetes/pkg/api/resource"
	conversion "k8s.io/kubernetes/pkg/conversion"
	util "k8s.io/kubernetes/pkg/util"
	inf "speter.net/go/exp/math/dec/inf"
)

func deepCopy_api_ListMeta(in api.ListMeta, out *api.ListMeta, c *conversion.Cloner) error {
	out.SelfLink = in.SelfLink
	out.ResourceVersion = in.ResourceVersion
	return nil
}

func deepCopy_api_ObjectMeta(in api.ObjectMeta, out *api.ObjectMeta, c *conversion.Cloner) error {
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := deepCopy_util_Time(in.CreationTimestamp, &out.CreationTimestamp, c); err != nil {
		return err
	}
	if in.DeletionTimestamp != nil {
		out.DeletionTimestamp = new(util.Time)
		if err := deepCopy_util_Time(*in.DeletionTimestamp, out.DeletionTimestamp, c); err != nil {
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

func deepCopy_api_TypeMeta(in api.TypeMeta, out *api.TypeMeta, c *conversion.Cloner) error {
	out.Kind = in.Kind
	out.APIVersion = in.APIVersion
	return nil
}

func deepCopy_resource_Quantity(in resource.Quantity, out *resource.Quantity, c *conversion.Cloner) error {
	if in.Amount != nil {
		if newVal, err := c.DeepCopy(in.Amount); err != nil {
			return err
		} else if newVal == nil {
			out.Amount = nil
		} else {
			out.Amount = newVal.(*inf.Dec)
		}
	} else {
		out.Amount = nil
	}
	out.Format = in.Format
	return nil
}

func deepCopy_expapi_HorizontalPodAutoscaler(in HorizontalPodAutoscaler, out *HorizontalPodAutoscaler, c *conversion.Cloner) error {
	if err := deepCopy_api_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := deepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if err := deepCopy_expapi_HorizontalPodAutoscalerSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_expapi_HorizontalPodAutoscalerList(in HorizontalPodAutoscalerList, out *HorizontalPodAutoscalerList, c *conversion.Cloner) error {
	if err := deepCopy_api_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := deepCopy_api_ListMeta(in.ListMeta, &out.ListMeta, c); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]HorizontalPodAutoscaler, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_expapi_HorizontalPodAutoscaler(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_expapi_HorizontalPodAutoscalerSpec(in HorizontalPodAutoscalerSpec, out *HorizontalPodAutoscalerSpec, c *conversion.Cloner) error {
	if in.ScaleRef != nil {
		out.ScaleRef = new(SubresourceReference)
		if err := deepCopy_expapi_SubresourceReference(*in.ScaleRef, out.ScaleRef, c); err != nil {
			return err
		}
	} else {
		out.ScaleRef = nil
	}
	out.MinCount = in.MinCount
	out.MaxCount = in.MaxCount
	if err := deepCopy_expapi_TargetConsumption(in.Target, &out.Target, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_expapi_ReplicationControllerDummy(in ReplicationControllerDummy, out *ReplicationControllerDummy, c *conversion.Cloner) error {
	if err := deepCopy_api_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_expapi_Scale(in Scale, out *Scale, c *conversion.Cloner) error {
	if err := deepCopy_api_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := deepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if err := deepCopy_expapi_ScaleSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_expapi_ScaleStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_expapi_ScaleSpec(in ScaleSpec, out *ScaleSpec, c *conversion.Cloner) error {
	out.Replicas = in.Replicas
	return nil
}

func deepCopy_expapi_ScaleStatus(in ScaleStatus, out *ScaleStatus, c *conversion.Cloner) error {
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

func deepCopy_expapi_SubresourceReference(in SubresourceReference, out *SubresourceReference, c *conversion.Cloner) error {
	out.Kind = in.Kind
	out.Namespace = in.Namespace
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	out.Subresource = in.Subresource
	return nil
}

func deepCopy_expapi_TargetConsumption(in TargetConsumption, out *TargetConsumption, c *conversion.Cloner) error {
	out.Resource = in.Resource
	if err := deepCopy_resource_Quantity(in.Quantity, &out.Quantity, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_util_Time(in util.Time, out *util.Time, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.Time); err != nil {
		return err
	} else {
		out.Time = newVal.(time.Time)
	}
	return nil
}

func init() {
	err := api.Scheme.AddGeneratedDeepCopyFuncs(
		deepCopy_api_ListMeta,
		deepCopy_api_ObjectMeta,
		deepCopy_api_TypeMeta,
		deepCopy_resource_Quantity,
		deepCopy_expapi_HorizontalPodAutoscaler,
		deepCopy_expapi_HorizontalPodAutoscalerList,
		deepCopy_expapi_HorizontalPodAutoscalerSpec,
		deepCopy_expapi_ReplicationControllerDummy,
		deepCopy_expapi_Scale,
		deepCopy_expapi_ScaleSpec,
		deepCopy_expapi_ScaleStatus,
		deepCopy_expapi_SubresourceReference,
		deepCopy_expapi_TargetConsumption,
		deepCopy_util_Time,
	)
	if err != nil {
		// if one of the deep copy functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
