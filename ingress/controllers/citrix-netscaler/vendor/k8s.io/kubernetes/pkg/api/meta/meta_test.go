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

package meta_test

import (
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/meta/metatypes"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
)

func TestAPIObjectMeta(t *testing.T) {
	j := &api.Pod{
		TypeMeta: unversioned.TypeMeta{APIVersion: "/a", Kind: "b"},
		ObjectMeta: api.ObjectMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"x": "y"},
			Finalizers: []string{
				"finalizer.1",
				"finalizer.2",
			},
		},
	}
	var _ meta.Object = &j.ObjectMeta
	var _ meta.ObjectMetaAccessor = j
	accessor, err := meta.Accessor(j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessor != meta.Object(&j.ObjectMeta) {
		t.Fatalf("should have returned the same pointer: %#v %#v", accessor, j)
	}
	if e, a := "bar", accessor.GetNamespace(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "foo", accessor.GetName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "prefix", accessor.GetGenerateName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "uid", string(accessor.GetUID()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "1", accessor.GetResourceVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "some/place/only/we/know", accessor.GetSelfLink(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := []string{"finalizer.1", "finalizer.2"}, accessor.GetFinalizers(); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor, err := meta.TypeAccessor(j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "a", typeAccessor.GetAPIVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "b", typeAccessor.GetKind(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	accessor.SetNamespace("baz")
	accessor.SetName("bar")
	accessor.SetGenerateName("generate")
	accessor.SetUID("other")
	typeAccessor.SetAPIVersion("c")
	typeAccessor.SetKind("d")
	accessor.SetResourceVersion("2")
	accessor.SetSelfLink("google.com")
	accessor.SetFinalizers([]string{"finalizer.3"})

	// Prove that accessor changes the original object.
	if e, a := "baz", j.Namespace; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "bar", j.Name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "generate", j.GenerateName; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := types.UID("other"), j.UID; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "c", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "d", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "2", j.ResourceVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "google.com", j.SelfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := []string{"finalizer.3"}, j.Finalizers; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor.SetAPIVersion("d")
	typeAccessor.SetKind("e")
	if e, a := "d", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "e", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestGenericTypeMeta(t *testing.T) {
	type TypeMeta struct {
		Kind              string               `json:"kind,omitempty"`
		Namespace         string               `json:"namespace,omitempty"`
		Name              string               `json:"name,omitempty"`
		GenerateName      string               `json:"generateName,omitempty"`
		UID               string               `json:"uid,omitempty"`
		CreationTimestamp unversioned.Time     `json:"creationTimestamp,omitempty"`
		SelfLink          string               `json:"selfLink,omitempty"`
		ResourceVersion   string               `json:"resourceVersion,omitempty"`
		APIVersion        string               `json:"apiVersion,omitempty"`
		Labels            map[string]string    `json:"labels,omitempty"`
		Annotations       map[string]string    `json:"annotations,omitempty"`
		OwnerReferences   []api.OwnerReference `json:"ownerReferences,omitempty"`
		Finalizers        []string             `json:"finalizers,omitempty"`
	}
	type Object struct {
		TypeMeta `json:",inline"`
	}
	j := Object{
		TypeMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			APIVersion:      "a",
			Kind:            "b",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"x": "y"},
			Finalizers:      []string{"finalizer.1", "finalizer.2"},
		},
	}
	accessor, err := meta.Accessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "bar", accessor.GetNamespace(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "foo", accessor.GetName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "prefix", accessor.GetGenerateName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "uid", string(accessor.GetUID()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "1", accessor.GetResourceVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "some/place/only/we/know", accessor.GetSelfLink(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := []string{"finalizer.1", "finalizer.2"}, accessor.GetFinalizers(); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor, err := meta.TypeAccessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "a", typeAccessor.GetAPIVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "b", typeAccessor.GetKind(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	accessor.SetNamespace("baz")
	accessor.SetName("bar")
	accessor.SetGenerateName("generate")
	accessor.SetUID("other")
	typeAccessor.SetAPIVersion("c")
	typeAccessor.SetKind("d")
	accessor.SetResourceVersion("2")
	accessor.SetSelfLink("google.com")
	accessor.SetFinalizers([]string{"finalizer.3"})

	// Prove that accessor changes the original object.
	if e, a := "baz", j.Namespace; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "bar", j.Name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "generate", j.GenerateName; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "other", j.UID; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "c", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "d", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "2", j.ResourceVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "google.com", j.SelfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := []string{"finalizer.3"}, j.Finalizers; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor.SetAPIVersion("d")
	typeAccessor.SetKind("e")
	if e, a := "d", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "e", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

type InternalTypeMeta struct {
	Kind              string               `json:"kind,omitempty"`
	Namespace         string               `json:"namespace,omitempty"`
	Name              string               `json:"name,omitempty"`
	GenerateName      string               `json:"generateName,omitempty"`
	UID               string               `json:"uid,omitempty"`
	CreationTimestamp unversioned.Time     `json:"creationTimestamp,omitempty"`
	SelfLink          string               `json:"selfLink,omitempty"`
	ResourceVersion   string               `json:"resourceVersion,omitempty"`
	APIVersion        string               `json:"apiVersion,omitempty"`
	Labels            map[string]string    `json:"labels,omitempty"`
	Annotations       map[string]string    `json:"annotations,omitempty"`
	Finalizers        []string             `json:"finalizers,omitempty"`
	OwnerReferences   []api.OwnerReference `json:"ownerReferences,omitempty"`
}

type InternalObject struct {
	TypeMeta InternalTypeMeta `json:",inline"`
}

func (obj *InternalObject) GetObjectKind() unversioned.ObjectKind { return obj }
func (obj *InternalObject) SetGroupVersionKind(gvk unversioned.GroupVersionKind) {
	obj.TypeMeta.APIVersion, obj.TypeMeta.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *InternalObject) GroupVersionKind() unversioned.GroupVersionKind {
	return unversioned.FromAPIVersionAndKind(obj.TypeMeta.APIVersion, obj.TypeMeta.Kind)
}

func TestGenericTypeMetaAccessor(t *testing.T) {
	j := &InternalObject{
		InternalTypeMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			APIVersion:      "/a",
			Kind:            "b",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"x": "y"},
			// OwnerReferences are tested separately
		},
	}
	accessor := meta.NewAccessor()
	namespace, err := accessor.Namespace(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "bar", namespace; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	name, err := accessor.Name(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "foo", name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	generateName, err := accessor.GenerateName(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "prefix", generateName; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	uid, err := accessor.UID(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "uid", string(uid); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	apiVersion, err := accessor.APIVersion(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "a", apiVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	kind, err := accessor.Kind(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "b", kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	rv, err := accessor.ResourceVersion(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "1", rv; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	selfLink, err := accessor.SelfLink(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := "some/place/only/we/know", selfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	labels, err := accessor.Labels(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := 1, len(labels); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	annotations, err := accessor.Annotations(j)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := 1, len(annotations); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	if err := accessor.SetNamespace(j, "baz"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetName(j, "bar"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetGenerateName(j, "generate"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetUID(j, "other"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetAPIVersion(j, "c"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetKind(j, "d"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetResourceVersion(j, "2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetSelfLink(j, "google.com"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := accessor.SetLabels(j, map[string]string{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	var nilMap map[string]string
	if err := accessor.SetAnnotations(j, nilMap); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Prove that accessor changes the original object.
	if e, a := "baz", j.TypeMeta.Namespace; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "bar", j.TypeMeta.Name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "generate", j.TypeMeta.GenerateName; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "other", j.TypeMeta.UID; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "c", j.TypeMeta.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "d", j.TypeMeta.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "2", j.TypeMeta.ResourceVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "google.com", j.TypeMeta.SelfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := map[string]string{}, j.TypeMeta.Labels; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}
	if e, a := nilMap, j.TypeMeta.Annotations; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}
}

func TestGenericObjectMeta(t *testing.T) {
	type TypeMeta struct {
		Kind       string `json:"kind,omitempty"`
		APIVersion string `json:"apiVersion,omitempty"`
	}
	type ObjectMeta struct {
		Namespace         string               `json:"namespace,omitempty"`
		Name              string               `json:"name,omitempty"`
		GenerateName      string               `json:"generateName,omitempty"`
		UID               string               `json:"uid,omitempty"`
		CreationTimestamp unversioned.Time     `json:"creationTimestamp,omitempty"`
		SelfLink          string               `json:"selfLink,omitempty"`
		ResourceVersion   string               `json:"resourceVersion,omitempty"`
		Labels            map[string]string    `json:"labels,omitempty"`
		Annotations       map[string]string    `json:"annotations,omitempty"`
		Finalizers        []string             `json:"finalizers,omitempty"`
		OwnerReferences   []api.OwnerReference `json:"ownerReferences,omitempty"`
	}
	type Object struct {
		TypeMeta   `json:",inline"`
		ObjectMeta `json:"metadata"`
	}
	j := Object{
		TypeMeta{
			APIVersion: "a",
			Kind:       "b",
		},
		ObjectMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"a": "b"},
			Finalizers: []string{
				"finalizer.1",
				"finalizer.2",
			},
		},
	}
	accessor, err := meta.Accessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "bar", accessor.GetNamespace(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "foo", accessor.GetName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "prefix", accessor.GetGenerateName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "uid", string(accessor.GetUID()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "1", accessor.GetResourceVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "some/place/only/we/know", accessor.GetSelfLink(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(accessor.GetLabels()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(accessor.GetAnnotations()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := []string{"finalizer.1", "finalizer.2"}, accessor.GetFinalizers(); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor, err := meta.TypeAccessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "a", typeAccessor.GetAPIVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "b", typeAccessor.GetKind(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	accessor.SetNamespace("baz")
	accessor.SetName("bar")
	accessor.SetGenerateName("generate")
	accessor.SetUID("other")
	typeAccessor.SetAPIVersion("c")
	typeAccessor.SetKind("d")
	accessor.SetResourceVersion("2")
	accessor.SetSelfLink("google.com")
	accessor.SetLabels(map[string]string{"other": "label"})
	accessor.SetAnnotations(map[string]string{"c": "d"})
	accessor.SetFinalizers([]string{"finalizer.3"})

	// Prove that accessor changes the original object.
	if e, a := "baz", j.Namespace; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "bar", j.Name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "generate", j.GenerateName; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "other", j.UID; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "c", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "d", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "2", j.ResourceVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "google.com", j.SelfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := map[string]string{"other": "label"}, j.Labels; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}
	if e, a := map[string]string{"c": "d"}, j.Annotations; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}
	if e, a := []string{"finalizer.3"}, j.Finalizers; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestGenericListMeta(t *testing.T) {
	type TypeMeta struct {
		Kind       string `json:"kind,omitempty"`
		APIVersion string `json:"apiVersion,omitempty"`
	}
	type ListMeta struct {
		SelfLink        string `json:"selfLink,omitempty"`
		ResourceVersion string `json:"resourceVersion,omitempty"`
	}
	type Object struct {
		TypeMeta `json:",inline"`
		ListMeta `json:"metadata"`
	}
	j := Object{
		TypeMeta{
			APIVersion: "a",
			Kind:       "b",
		},
		ListMeta{
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
		},
	}
	accessor, err := meta.Accessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "", accessor.GetName(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "", string(accessor.GetUID()); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "1", accessor.GetResourceVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "some/place/only/we/know", accessor.GetSelfLink(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	typeAccessor, err := meta.TypeAccessor(&j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "a", typeAccessor.GetAPIVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "b", typeAccessor.GetKind(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	accessor.SetName("bar")
	accessor.SetUID("other")
	typeAccessor.SetAPIVersion("c")
	typeAccessor.SetKind("d")
	accessor.SetResourceVersion("2")
	accessor.SetSelfLink("google.com")

	// Prove that accessor changes the original object.
	if e, a := "c", j.APIVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "d", j.Kind; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "2", j.ResourceVersion; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "google.com", j.SelfLink; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

type MyAPIObject struct {
	TypeMeta InternalTypeMeta `json:",inline"`
}

func (obj *MyAPIObject) GetObjectKind() unversioned.ObjectKind { return obj }
func (obj *MyAPIObject) SetGroupVersionKind(gvk unversioned.GroupVersionKind) {
	obj.TypeMeta.APIVersion, obj.TypeMeta.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *MyAPIObject) GroupVersionKind() unversioned.GroupVersionKind {
	return unversioned.FromAPIVersionAndKind(obj.TypeMeta.APIVersion, obj.TypeMeta.Kind)
}

type MyIncorrectlyMarkedAsAPIObject struct{}

func (obj *MyIncorrectlyMarkedAsAPIObject) GetObjectKind() unversioned.ObjectKind {
	return unversioned.EmptyObjectKind
}

func TestResourceVersionerOfAPI(t *testing.T) {
	type T struct {
		runtime.Object
		Expected string
	}
	testCases := map[string]T{
		"empty api object":                   {&MyAPIObject{}, ""},
		"api object with version":            {&MyAPIObject{TypeMeta: InternalTypeMeta{ResourceVersion: "1"}}, "1"},
		"pointer to api object with version": {&MyAPIObject{TypeMeta: InternalTypeMeta{ResourceVersion: "1"}}, "1"},
	}
	versioning := meta.NewAccessor()
	for key, testCase := range testCases {
		actual, err := versioning.ResourceVersion(testCase.Object)
		if err != nil {
			t.Errorf("%s: unexpected error %#v", key, err)
		}
		if actual != testCase.Expected {
			t.Errorf("%s: expected %v, got %v", key, testCase.Expected, actual)
		}
	}

	failingCases := map[string]struct {
		runtime.Object
		Expected string
	}{
		"not a valid object to try": {&MyIncorrectlyMarkedAsAPIObject{}, "1"},
	}
	for key, testCase := range failingCases {
		_, err := versioning.ResourceVersion(testCase.Object)
		if err == nil {
			t.Errorf("%s: expected error, got nil", key)
		}
	}

	setCases := map[string]struct {
		runtime.Object
		Expected string
	}{
		"pointer to api object with version": {&MyAPIObject{TypeMeta: InternalTypeMeta{ResourceVersion: "1"}}, "1"},
	}
	for key, testCase := range setCases {
		if err := versioning.SetResourceVersion(testCase.Object, "5"); err != nil {
			t.Errorf("%s: unexpected error %#v", key, err)
		}
		actual, err := versioning.ResourceVersion(testCase.Object)
		if err != nil {
			t.Errorf("%s: unexpected error %#v", key, err)
		}
		if actual != "5" {
			t.Errorf("%s: expected %v, got %v", key, "5", actual)
		}
	}
}

func TestTypeMetaSelfLinker(t *testing.T) {
	table := map[string]struct {
		obj     runtime.Object
		expect  string
		try     string
		succeed bool
	}{
		"normal": {
			obj:     &MyAPIObject{TypeMeta: InternalTypeMeta{SelfLink: "foobar"}},
			expect:  "foobar",
			try:     "newbar",
			succeed: true,
		},
		"fail": {
			obj:     &MyIncorrectlyMarkedAsAPIObject{},
			succeed: false,
		},
	}

	linker := runtime.SelfLinker(meta.NewAccessor())
	for name, item := range table {
		got, err := linker.SelfLink(item.obj)
		if e, a := item.succeed, err == nil; e != a {
			t.Errorf("%v: expected %v, got %v", name, e, a)
		}
		if e, a := item.expect, got; item.succeed && e != a {
			t.Errorf("%v: expected %v, got %v", name, e, a)
		}

		err = linker.SetSelfLink(item.obj, item.try)
		if e, a := item.succeed, err == nil; e != a {
			t.Errorf("%v: expected %v, got %v", name, e, a)
		}
		if item.succeed {
			got, err := linker.SelfLink(item.obj)
			if err != nil {
				t.Errorf("%v: expected no err, got %v", name, err)
			}
			if e, a := item.try, got; e != a {
				t.Errorf("%v: expected %v, got %v", name, e, a)
			}
		}
	}
}

type MyAPIObject2 struct {
	unversioned.TypeMeta
	v1.ObjectMeta
}

func getObjectMetaAndOwnerRefereneces() (myAPIObject2 MyAPIObject2, metaOwnerReferences []metatypes.OwnerReference) {
	fuzz.New().NilChance(.5).NumElements(1, 5).Fuzz(&myAPIObject2)
	references := myAPIObject2.ObjectMeta.OwnerReferences
	// This is necessary for the test to pass because the getter will return a
	// non-nil slice.
	metaOwnerReferences = make([]metatypes.OwnerReference, 0)
	for i := 0; i < len(references); i++ {
		metaOwnerReferences = append(metaOwnerReferences, metatypes.OwnerReference{
			Kind:       references[i].Kind,
			Name:       references[i].Name,
			UID:        references[i].UID,
			APIVersion: references[i].APIVersion,
			Controller: references[i].Controller,
		})
	}
	if len(references) == 0 {
		// This is necessary for the test to pass because the setter will make a
		// non-nil slice.
		myAPIObject2.ObjectMeta.OwnerReferences = make([]v1.OwnerReference, 0)
	}
	return myAPIObject2, metaOwnerReferences
}

func testGetOwnerReferences(t *testing.T) {
	obj, expected := getObjectMetaAndOwnerRefereneces()
	accessor, err := meta.Accessor(&obj)
	if err != nil {
		t.Error(err)
	}
	references := accessor.GetOwnerReferences()
	if !reflect.DeepEqual(references, expected) {
		t.Errorf("expect %#v\n got %#v", expected, references)
	}
}

func testSetOwnerReferences(t *testing.T) {
	expected, references := getObjectMetaAndOwnerRefereneces()
	obj := MyAPIObject2{}
	accessor, err := meta.Accessor(&obj)
	if err != nil {
		t.Error(err)
	}
	accessor.SetOwnerReferences(references)
	if e, a := expected.ObjectMeta.OwnerReferences, obj.ObjectMeta.OwnerReferences; !reflect.DeepEqual(e, a) {
		t.Errorf("expect %#v\n got %#v", e, a)
	}
}

func TestAccessOwnerReferences(t *testing.T) {
	fuzzIter := 5
	for i := 0; i < fuzzIter; i++ {
		testGetOwnerReferences(t)
		testSetOwnerReferences(t)
	}
}

// BenchmarkAccessorSetFastPath shows the interface fast path
func BenchmarkAccessorSetFastPath(b *testing.B) {
	obj := &api.Pod{
		TypeMeta: unversioned.TypeMeta{APIVersion: "/a", Kind: "b"},
		ObjectMeta: api.ObjectMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"x": "y"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc, err := meta.Accessor(obj)
		if err != nil {
			b.Fatal(err)
		}
		acc.SetNamespace("something")
	}
	b.StopTimer()
}

// BenchmarkAccessorSetReflection provides a baseline for accessor performance
func BenchmarkAccessorSetReflection(b *testing.B) {
	obj := &InternalObject{
		InternalTypeMeta{
			Namespace:       "bar",
			Name:            "foo",
			GenerateName:    "prefix",
			UID:             "uid",
			APIVersion:      "a",
			Kind:            "b",
			ResourceVersion: "1",
			SelfLink:        "some/place/only/we/know",
			Labels:          map[string]string{"foo": "bar"},
			Annotations:     map[string]string{"x": "y"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc, err := meta.Accessor(obj)
		if err != nil {
			b.Fatal(err)
		}
		acc.SetNamespace("something")
	}
	b.StopTimer()
}
