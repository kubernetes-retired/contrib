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

package kubectl

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	Interval = time.Second * 1
	Timeout  = time.Minute * 5
)

// A Reaper handles terminating an object as gracefully as possible.
// timeout is how long we'll wait for the termination to be successful
// gracePeriod is time given to an API object for it to delete itself cleanly (e.g. pod shutdown)
type Reaper interface {
	Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error
}

type NoSuchReaperError struct {
	kind unversioned.GroupKind
}

func (n *NoSuchReaperError) Error() string {
	return fmt.Sprintf("no reaper has been implemented for %v", n.kind)
}

func IsNoSuchReaperError(err error) bool {
	_, ok := err.(*NoSuchReaperError)
	return ok
}

func ReaperFor(kind unversioned.GroupKind, c client.Interface) (Reaper, error) {
	switch kind {
	case api.Kind("ReplicationController"):
		return &ReplicationControllerReaper{c, Interval, Timeout}, nil

	case extensions.Kind("DaemonSet"):
		return &DaemonSetReaper{c, Interval, Timeout}, nil

	case api.Kind("Pod"):
		return &PodReaper{c}, nil

	case api.Kind("Service"):
		return &ServiceReaper{c}, nil

	case extensions.Kind("Job"):
		return &JobReaper{c, Interval, Timeout}, nil

	}
	return nil, &NoSuchReaperError{kind}
}

func ReaperForReplicationController(c client.Interface, timeout time.Duration) (Reaper, error) {
	return &ReplicationControllerReaper{c, Interval, timeout}, nil
}

type ReplicationControllerReaper struct {
	client.Interface
	pollInterval, timeout time.Duration
}
type DaemonSetReaper struct {
	client.Interface
	pollInterval, timeout time.Duration
}
type JobReaper struct {
	client.Interface
	pollInterval, timeout time.Duration
}
type PodReaper struct {
	client.Interface
}
type ServiceReaper struct {
	client.Interface
}

type objInterface interface {
	Delete(name string) error
	Get(name string) (meta.Object, error)
}

// getOverlappingControllers finds rcs that this controller overlaps, as well as rcs overlapping this controller.
func getOverlappingControllers(c client.ReplicationControllerInterface, rc *api.ReplicationController) ([]api.ReplicationController, error) {
	rcs, err := c.List(api.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting replication controllers: %v", err)
	}
	var matchingRCs []api.ReplicationController
	rcLabels := labels.Set(rc.Spec.Selector)
	for _, controller := range rcs.Items {
		newRCLabels := labels.Set(controller.Spec.Selector)
		if labels.SelectorFromSet(newRCLabels).Matches(rcLabels) || labels.SelectorFromSet(rcLabels).Matches(newRCLabels) {
			matchingRCs = append(matchingRCs, controller)
		}
	}
	return matchingRCs, nil
}

func (reaper *ReplicationControllerReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	rc := reaper.ReplicationControllers(namespace)
	scaler, err := ScalerFor(api.Kind("ReplicationController"), *reaper)
	if err != nil {
		return err
	}
	ctrl, err := rc.Get(name)
	if err != nil {
		return err
	}
	if timeout == 0 {
		timeout = Timeout + time.Duration(10*ctrl.Spec.Replicas)*time.Second
	}

	// The rc manager will try and detect all matching rcs for a pod's labels,
	// and only sync the oldest one. This means if we have a pod with labels
	// [(k1: v1), (k2: v2)] and two rcs: rc1 with selector [(k1=v1)], and rc2 with selector [(k1=v1),(k2=v2)],
	// the rc manager will sync the older of the two rcs.
	//
	// If there are rcs with a superset of labels, eg:
	// deleting: (k1=v1), superset: (k2=v2, k1=v1)
	//	- It isn't safe to delete the rc because there could be a pod with labels
	//	  (k1=v1) that isn't managed by the superset rc. We can't scale it down
	//	  either, because there could be a pod (k2=v2, k1=v1) that it deletes
	//	  causing a fight with the superset rc.
	// If there are rcs with a subset of labels, eg:
	// deleting: (k2=v2, k1=v1), subset: (k1=v1), superset: (k2=v2, k1=v1, k3=v3)
	//  - Even if it's safe to delete this rc without a scale down because all it's pods
	//	  are being controlled by the subset rc the code returns an error.

	// In theory, creating overlapping controllers is user error, so the loop below
	// tries to account for this logic only in the common case, where we end up
	// with multiple rcs that have an exact match on selectors.

	overlappingCtrls, err := getOverlappingControllers(rc, ctrl)
	if err != nil {
		return fmt.Errorf("error getting replication controllers: %v", err)
	}
	exactMatchRCs := []api.ReplicationController{}
	overlapRCs := []string{}
	for _, overlappingRC := range overlappingCtrls {
		if len(overlappingRC.Spec.Selector) == len(ctrl.Spec.Selector) {
			exactMatchRCs = append(exactMatchRCs, overlappingRC)
		} else {
			overlapRCs = append(overlapRCs, overlappingRC.Name)
		}
	}
	if len(overlapRCs) > 0 {
		return fmt.Errorf(
			"Detected overlapping controllers for rc %v: %v, please manage deletion individually with --cascade=false.",
			ctrl.Name, strings.Join(overlapRCs, ","))
	}
	if len(exactMatchRCs) == 1 {
		// No overlapping controllers.
		retry := NewRetryParams(reaper.pollInterval, reaper.timeout)
		waitForReplicas := NewRetryParams(reaper.pollInterval, timeout)
		if err = scaler.Scale(namespace, name, 0, nil, retry, waitForReplicas); err != nil {
			return err
		}
	}
	if err := rc.Delete(name); err != nil {
		return err
	}
	return nil
}

func (reaper *DaemonSetReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	ds, err := reaper.Extensions().DaemonSets(namespace).Get(name)
	if err != nil {
		return err
	}

	// We set the nodeSelector to a random label. This label is nearly guaranteed
	// to not be set on any node so the DameonSetController will start deleting
	// daemon pods. Once it's done deleting the daemon pods, it's safe to delete
	// the DaemonSet.
	ds.Spec.Template.Spec.NodeSelector = map[string]string{
		string(util.NewUUID()): string(util.NewUUID()),
	}
	// force update to avoid version conflict
	ds.ResourceVersion = ""

	if ds, err = reaper.Extensions().DaemonSets(namespace).Update(ds); err != nil {
		return err
	}

	// Wait for the daemon set controller to kill all the daemon pods.
	if err := wait.Poll(reaper.pollInterval, reaper.timeout, func() (bool, error) {
		updatedDS, err := reaper.Extensions().DaemonSets(namespace).Get(name)
		if err != nil {
			return false, nil
		}
		return updatedDS.Status.CurrentNumberScheduled+updatedDS.Status.NumberMisscheduled == 0, nil
	}); err != nil {
		return err
	}

	if err := reaper.Extensions().DaemonSets(namespace).Delete(name); err != nil {
		return err
	}
	return nil
}

func (reaper *JobReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	jobs := reaper.Extensions().Jobs(namespace)
	pods := reaper.Pods(namespace)
	scaler, err := ScalerFor(extensions.Kind("Job"), *reaper)
	if err != nil {
		return err
	}
	job, err := jobs.Get(name)
	if err != nil {
		return err
	}
	if timeout == 0 {
		// we will never have more active pods than job.Spec.Parallelism
		parallelism := *job.Spec.Parallelism
		timeout = Timeout + time.Duration(10*parallelism)*time.Second
	}

	// TODO: handle overlapping jobs
	retry := NewRetryParams(reaper.pollInterval, reaper.timeout)
	waitForJobs := NewRetryParams(reaper.pollInterval, timeout)
	if err = scaler.Scale(namespace, name, 0, nil, retry, waitForJobs); err != nil {
		return err
	}
	// at this point only dead pods are left, that should be removed
	selector, _ := extensions.LabelSelectorAsSelector(job.Spec.Selector)
	options := api.ListOptions{LabelSelector: selector}
	podList, err := pods.List(options)
	if err != nil {
		return err
	}
	errList := []error{}
	for _, pod := range podList.Items {
		if err := pods.Delete(pod.Name, gracePeriod); err != nil {
			// ignores the error when the pod isn't found
			if !errors.IsNotFound(err) {
				errList = append(errList, err)
			}
		}
	}
	if len(errList) > 0 {
		return utilerrors.NewAggregate(errList)
	}
	// once we have all the pods removed we can safely remove the job itself
	if err := jobs.Delete(name, gracePeriod); err != nil {
		return err
	}
	return nil
}

func (reaper *PodReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	pods := reaper.Pods(namespace)
	_, err := pods.Get(name)
	if err != nil {
		return err
	}
	if err := pods.Delete(name, gracePeriod); err != nil {
		return err
	}

	return nil
}

func (reaper *ServiceReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	services := reaper.Services(namespace)
	_, err := services.Get(name)
	if err != nil {
		return err
	}
	if err := services.Delete(name); err != nil {
		return err
	}
	return nil
}
