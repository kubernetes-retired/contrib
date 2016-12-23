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

package fake

import (
	api "k8s.io/kubernetes/pkg/api"
	v1 "k8s.io/kubernetes/pkg/api/v1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	schema "k8s.io/kubernetes/pkg/runtime/schema"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakePersistentVolumes implements PersistentVolumeInterface
type FakePersistentVolumes struct {
	Fake *FakeCoreV1
}

var persistentvolumesResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}

func (c *FakePersistentVolumes) Create(persistentVolume *v1.PersistentVolume) (result *v1.PersistentVolume, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(persistentvolumesResource, persistentVolume), &v1.PersistentVolume{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PersistentVolume), err
}

func (c *FakePersistentVolumes) Update(persistentVolume *v1.PersistentVolume) (result *v1.PersistentVolume, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(persistentvolumesResource, persistentVolume), &v1.PersistentVolume{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PersistentVolume), err
}

func (c *FakePersistentVolumes) UpdateStatus(persistentVolume *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateSubresourceAction(persistentvolumesResource, "status", persistentVolume), &v1.PersistentVolume{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PersistentVolume), err
}

func (c *FakePersistentVolumes) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(persistentvolumesResource, name), &v1.PersistentVolume{})
	return err
}

func (c *FakePersistentVolumes) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(persistentvolumesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.PersistentVolumeList{})
	return err
}

func (c *FakePersistentVolumes) Get(name string) (result *v1.PersistentVolume, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(persistentvolumesResource, name), &v1.PersistentVolume{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PersistentVolume), err
}

func (c *FakePersistentVolumes) List(opts v1.ListOptions) (result *v1.PersistentVolumeList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(persistentvolumesResource, opts), &v1.PersistentVolumeList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.PersistentVolumeList{}
	for _, item := range obj.(*v1.PersistentVolumeList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested persistentVolumes.
func (c *FakePersistentVolumes) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(persistentvolumesResource, opts))
}

// Patch applies the patch and returns the patched persistentVolume.
func (c *FakePersistentVolumes) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.PersistentVolume, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(persistentvolumesResource, name, data, subresources...), &v1.PersistentVolume{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PersistentVolume), err
}
