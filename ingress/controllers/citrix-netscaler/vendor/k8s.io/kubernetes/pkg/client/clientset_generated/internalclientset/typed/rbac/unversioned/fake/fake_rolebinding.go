/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	rbac "k8s.io/kubernetes/pkg/apis/rbac"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeRoleBindings implements RoleBindingInterface
type FakeRoleBindings struct {
	Fake *FakeRbac
	ns   string
}

var rolebindingsResource = unversioned.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "", Resource: "rolebindings"}

func (c *FakeRoleBindings) Create(roleBinding *rbac.RoleBinding) (result *rbac.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(rolebindingsResource, c.ns, roleBinding), &rbac.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*rbac.RoleBinding), err
}

func (c *FakeRoleBindings) Update(roleBinding *rbac.RoleBinding) (result *rbac.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(rolebindingsResource, c.ns, roleBinding), &rbac.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*rbac.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(rolebindingsResource, c.ns, name), &rbac.RoleBinding{})

	return err
}

func (c *FakeRoleBindings) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(rolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &rbac.RoleBindingList{})
	return err
}

func (c *FakeRoleBindings) Get(name string) (result *rbac.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(rolebindingsResource, c.ns, name), &rbac.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*rbac.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts api.ListOptions) (result *rbac.RoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(rolebindingsResource, c.ns, opts), &rbac.RoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &rbac.RoleBindingList{}
	for _, item := range obj.(*rbac.RoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roleBindings.
func (c *FakeRoleBindings) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(rolebindingsResource, c.ns, opts))

}
