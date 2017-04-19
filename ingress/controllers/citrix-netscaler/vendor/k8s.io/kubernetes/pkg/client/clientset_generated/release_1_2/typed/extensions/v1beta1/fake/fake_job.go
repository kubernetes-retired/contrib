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
	v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeJobs implements JobInterface
type FakeJobs struct {
	Fake *FakeExtensions
	ns   string
}

var jobsResource = unversioned.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "jobs"}

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

func (c *FakeJobs) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(jobsResource, c.ns, name), &v1beta1.Job{})

	return err
}

func (c *FakeJobs) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
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

func (c *FakeJobs) List(opts api.ListOptions) (result *v1beta1.JobList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(jobsResource, c.ns, opts), &v1beta1.JobList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeJobs) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(jobsResource, c.ns, opts))

}
