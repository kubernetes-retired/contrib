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

package main

import (
	"net/http"
	"strings"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
)

// Instances implements InstancePool.
type Instances struct {
	cloud     InstanceGroups
	defaultIG *compute.InstanceGroup

	// Currently unused. The current state for instances is derived from a
	// kubernetes nodeLister. All services as ports added to a single
	// instance group, the defaultIG. When we support adding an instance
	// to multiple instance groups, we can move the createInstanceGroup
	// method into the interface and use the poolStore.
	pool *poolStore
}

// createInstanceGroup creates an instance group.
func createInstanceGroup(cloud InstanceGroups, name string) (*compute.InstanceGroup, error) {
	ig, err := cloud.GetInstanceGroup(name)
	if ig != nil {
		glog.Infof("Instance group %v already exists", ig.Name)
		return ig, nil
	}

	glog.Infof("Creating instance group %v", name)
	ig, err = cloud.CreateInstanceGroup(name)
	if err != nil {
		return nil, err
	}
	return ig, err
}

// NewNodePool creates a new node pool.
// - cloud: implements InstanceGroups, used to sync Kubernetes nodes with
//   members of the cloud InstanceGroup identified by defaultIGName.
// - defaultIGName: Name of a GCE Instance Group with all nodes in your cluster.
//	 If this Instance Group doesn't exist, it will be created, if it does, it
//   will be reused.
func NewNodePool(cloud InstanceGroups, defaultIGName string) (NodePool, error) {
	// Each node pool has to have at least one default instance group backing
	// it in the cloud. We currently don't support pools of instance groups,
	// which is why createInstanceGroup is a private method invoked in the
	// constructor.
	ig, err := createInstanceGroup(cloud, defaultIGName)
	if err != nil {
		return nil, err
	}

	instances := &Instances{cloud, ig, newPoolStore()}
	instances.defaultIG = ig
	return instances, nil
}

func (i *Instances) list() (sets.String, error) {
	nodeNames := sets.NewString()
	instances, err := i.cloud.ListInstancesInInstanceGroup(
		i.defaultIG.Name, allInstances)
	if err != nil {
		return nodeNames, err
	}
	for _, ins := range instances.Items {
		// TODO: If round trips weren't so slow one would be inclided
		// to GetInstance using this url and get the name.
		parts := strings.Split(ins.Instance, "/")
		nodeNames.Insert(parts[len(parts)-1])
	}
	return nodeNames, nil
}

// Get returns the Instance Group by name.
func (i *Instances) Get(name string) (*compute.InstanceGroup, error) {
	ig, err := i.cloud.GetInstanceGroup(name)
	if err != nil {
		return nil, err
	}
	return ig, nil
}

// Add adds the given instances to the Instance Group.
func (i *Instances) Add(names []string) error {
	glog.Infof("Adding nodes %v to %v", names, i.defaultIG.Name)
	return i.cloud.AddInstancesToInstanceGroup(i.defaultIG.Name, names)
}

// Remove removes the given instances from the Instance Group.
func (i *Instances) Remove(names []string) error {
	glog.Infof("Removing nodes %v", names)
	return i.cloud.RemoveInstancesFromInstanceGroup(i.defaultIG.Name, names)
}

// Sync syncs kubernetes instances with the instances in the instance group.
func (i *Instances) Sync(nodes []string) (err error) {
	glog.Infof("Syncing nodes %v", nodes)
	defer func() {
		// If the default Instance group doesn't exist because someone
		// messed with the UI, recreate it.
		if err != nil && isHTTPErrorCode(err, http.StatusNotFound) {
			var ig *compute.InstanceGroup
			ig, err = createInstanceGroup(i.cloud, i.defaultIG.Name)
			if err == nil {
				i.defaultIG = ig
			}
		}
	}()

	gceNodes := sets.NewString()
	gceNodes, err = i.list()
	if err != nil {
		return err
	}
	kubeNodes := sets.NewString(nodes...)

	// A node deleted via kubernetes could still exist as a gce vm. We don't
	// want to route requests to it. Similarly, a node added to kubernetes
	// needs to get added to the instance group so we do route requests to it.

	removeNodes := gceNodes.Difference(kubeNodes).List()
	addNodes := kubeNodes.Difference(gceNodes).List()

	if len(removeNodes) != 0 {
		if err = i.Remove(
			gceNodes.Difference(kubeNodes).List()); err != nil {
			return err
		}
	}

	if len(addNodes) != 0 {
		if err = i.Add(
			kubeNodes.Difference(gceNodes).List()); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown deletes the default Instance Group.
func (i *Instances) Shutdown() error {
	return i.cloud.DeleteInstanceGroup(i.defaultIG.Name)
}
