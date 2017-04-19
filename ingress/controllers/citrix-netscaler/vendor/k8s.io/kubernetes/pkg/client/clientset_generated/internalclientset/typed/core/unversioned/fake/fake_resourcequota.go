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
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeResourceQuotas implements ResourceQuotaInterface
type FakeResourceQuotas struct {
	Fake *FakeCore
	ns   string
}

var resourcequotasResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "resourcequotas"}

func (c *FakeResourceQuotas) Create(resourceQuota *api.ResourceQuota) (result *api.ResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(resourcequotasResource, c.ns, resourceQuota), &api.ResourceQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ResourceQuota), err
}

func (c *FakeResourceQuotas) Update(resourceQuota *api.ResourceQuota) (result *api.ResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(resourcequotasResource, c.ns, resourceQuota), &api.ResourceQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ResourceQuota), err
}

func (c *FakeResourceQuotas) UpdateStatus(resourceQuota *api.ResourceQuota) (*api.ResourceQuota, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(resourcequotasResource, "status", c.ns, resourceQuota), &api.ResourceQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ResourceQuota), err
}

func (c *FakeResourceQuotas) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(resourcequotasResource, c.ns, name), &api.ResourceQuota{})

	return err
}

func (c *FakeResourceQuotas) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(resourcequotasResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ResourceQuotaList{})
	return err
}

func (c *FakeResourceQuotas) Get(name string) (result *api.ResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(resourcequotasResource, c.ns, name), &api.ResourceQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ResourceQuota), err
}

func (c *FakeResourceQuotas) List(opts api.ListOptions) (result *api.ResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(resourcequotasResource, c.ns, opts), &api.ResourceQuotaList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ResourceQuotaList{}
	for _, item := range obj.(*api.ResourceQuotaList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested resourceQuotas.
func (c *FakeResourceQuotas) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(resourcequotasResource, c.ns, opts))

}
