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
	v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	schema "k8s.io/kubernetes/pkg/runtime/schema"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeJobs implements JobInterface
type FakeJobs struct {
	Fake *FakeExtensionsV1beta1
	ns   string
}

var jobsResource = schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "jobs"}

func (c *FakeJobs) Create(job *v1beta1.Job) (result *v1beta1.Job, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(jobsResource, c.ns, job), &v1beta1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Job), err
}

func (c *FakeJobs) Update(job *v1beta1.Job) (result *v1beta1.Job, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(jobsResource, c.ns, job), &v1beta1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Job), err
}

func (c *FakeJobs) UpdateStatus(job *v1beta1.Job) (*v1beta1.Job, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(jobsResource, "status", c.ns, job), &v1beta1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Job), err
}

func (c *FakeJobs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(jobsResource, c.ns, name), &v1beta1.Job{})

	return err
}

func (c *FakeJobs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := core.NewDeleteCollectionAction(jobsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1beta1.JobList{})
	return err
}

func (c *FakeJobs) Get(name string) (result *v1beta1.Job, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(jobsResource, c.ns, name), &v1beta1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Job), err
}

func (c *FakeJobs) List(opts v1.ListOptions) (result *v1beta1.JobList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(jobsResource, c.ns, opts), &v1beta1.JobList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.JobList{}
	for _, item := range obj.(*v1beta1.JobList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested jobs.
func (c *FakeJobs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(jobsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched job.
func (c *FakeJobs) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1beta1.Job, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(jobsResource, c.ns, name, data, subresources...), &v1beta1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Job), err
}
