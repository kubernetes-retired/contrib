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

package v1beta1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// GroupName is the group name use in this package
const GroupName = "extensions"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1beta1"}

var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion)

func AddToScheme(scheme *runtime.Scheme) {
	addKnownTypes(scheme)
	addDefaultingFuncs(scheme)
	addConversionFuncs(scheme)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterAutoscaler{},
		&ClusterAutoscalerList{},
		&Deployment{},
		&DeploymentList{},
		&HorizontalPodAutoscaler{},
		&HorizontalPodAutoscalerList{},
		&Job{},
		&JobList{},
		&ReplicationControllerDummy{},
		&Scale{},
		&ThirdPartyResource{},
		&ThirdPartyResourceList{},
		&DaemonSetList{},
		&DaemonSet{},
		&ThirdPartyResourceData{},
		&ThirdPartyResourceDataList{},
		&Ingress{},
		&IngressList{},
		&ListOptions{},
		&ConfigMap{},
		&ConfigMapList{},
		&v1.DeleteOptions{},
	)
}

func (obj *ClusterAutoscaler) GetObjectKind() unversioned.ObjectKind           { return &obj.TypeMeta }
func (obj *ClusterAutoscalerList) GetObjectKind() unversioned.ObjectKind       { return &obj.TypeMeta }
func (obj *Deployment) GetObjectKind() unversioned.ObjectKind                  { return &obj.TypeMeta }
func (obj *DeploymentList) GetObjectKind() unversioned.ObjectKind              { return &obj.TypeMeta }
func (obj *HorizontalPodAutoscaler) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *HorizontalPodAutoscalerList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *Job) GetObjectKind() unversioned.ObjectKind                         { return &obj.TypeMeta }
func (obj *JobList) GetObjectKind() unversioned.ObjectKind                     { return &obj.TypeMeta }
func (obj *ReplicationControllerDummy) GetObjectKind() unversioned.ObjectKind  { return &obj.TypeMeta }
func (obj *Scale) GetObjectKind() unversioned.ObjectKind                       { return &obj.TypeMeta }
func (obj *ThirdPartyResource) GetObjectKind() unversioned.ObjectKind          { return &obj.TypeMeta }
func (obj *ThirdPartyResourceList) GetObjectKind() unversioned.ObjectKind      { return &obj.TypeMeta }
func (obj *DaemonSet) GetObjectKind() unversioned.ObjectKind                   { return &obj.TypeMeta }
func (obj *DaemonSetList) GetObjectKind() unversioned.ObjectKind               { return &obj.TypeMeta }
func (obj *ThirdPartyResourceData) GetObjectKind() unversioned.ObjectKind      { return &obj.TypeMeta }
func (obj *ThirdPartyResourceDataList) GetObjectKind() unversioned.ObjectKind  { return &obj.TypeMeta }
func (obj *Ingress) GetObjectKind() unversioned.ObjectKind                     { return &obj.TypeMeta }
func (obj *IngressList) GetObjectKind() unversioned.ObjectKind                 { return &obj.TypeMeta }
func (obj *ListOptions) GetObjectKind() unversioned.ObjectKind                 { return &obj.TypeMeta }
func (obj *ConfigMap) GetObjectKind() unversioned.ObjectKind                   { return &obj.TypeMeta }
func (obj *ConfigMapList) GetObjectKind() unversioned.ObjectKind               { return &obj.TypeMeta }
