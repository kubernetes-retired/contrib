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

/*
Package cache implements data structures used by the attach/detach controller
to keep track of volumes, the nodes they are attached to, and the pods that
reference them.
*/
package cache

import (
	"fmt"
	"sync"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util/attachdetach"
)

// DesiredStateOfWorld defines a set of thread-safe operations supported on
// the attach/detach controller's desired state of the world cache.
// This cache contains nodes->volumes->pods where nodes are all the nodes
// managed by the attach/detach controller, volumes are all the volumes that
// should be attached to the specified node, and pods are the pods that
// reference the volume and are scheduled to that node.
type DesiredStateOfWorld interface {
	// AddNode adds the given node to the list of nodes managed by the attach/
	// detach controller.
	// If the node already exists this is a no-op.
	AddNode(nodeName string)

	// AddPod adds the given pod to the list of pods that reference the
	// specified volume and is scheduled to the specified node.
	// A unique volumeName is generated from the volumeSpec and returned on
	// success.
	// If the pod already exists under the specified volume, this is a no-op.
	// If volumeSpec is not an attachable volume plugin, an error is returned.
	// If no volume with the name volumeName exists in the list of volumes that
	// should be attached to the specified node, the volume is implicitly added.
	// If no node with the name nodeName exists in list of nodes managed by the
	// attach/detach attached controller, an error is returned.
	AddPod(podName string, volumeSpec *volume.Spec, nodeName string) (api.UniqueDeviceName, error)

	// DeleteNode removes the given node from the list of nodes managed by the
	// attach/detach controller.
	// If the node does not exist this is a no-op.
	// If the node exists but has 1 or more child volumes, an error is returned.
	DeleteNode(nodeName string) error

	// DeletePod removes the given pod from the list of pods that reference the
	// specified volume and are scheduled to the specified node.
	// If no pod exists in the list of pods that reference the specified volume
	// and are scheduled to the specified node, this is a no-op.
	// If a node with the name nodeName does not exist in the list of nodes
	// managed by the attach/detach attached controller, this is a no-op.
	// If no volume with the name volumeName exists in the list of managed
	// volumes under the specified node, this is a no-op.
	// If after deleting the pod, the specified volume contains no other child
	// pods, the volume is also deleted.
	DeletePod(podName string, volumeName api.UniqueDeviceName, nodeName string)

	// NodeExists returns true if the node with the specified name exists in
	// the list of nodes managed by the attach/detach controller.
	NodeExists(nodeName string) bool

	// VolumeExists returns true if the volume with the specified name exists
	// in the list of volumes that should be attached to the specified node by
	// the attach detach controller.
	VolumeExists(volumeName api.UniqueDeviceName, nodeName string) bool

	// GetVolumesToAttach generates and returns a list of volumes to attach
	// and the nodes they should be attached to based on the current desired
	// state of the world.
	GetVolumesToAttach() []VolumeToAttach
}

// VolumeToAttach represents a volume that should be attached to a node.
type VolumeToAttach struct {
	// VolumeName is the unique identifier for the volume that should be
	// attached.
	VolumeName api.UniqueDeviceName

	// VolumeSpec is a volume spec containing the specification for the volume
	// that should be attached.
	VolumeSpec *volume.Spec

	// NodeName is the identifier for the node that the volume should be
	// attached to.
	NodeName string
}

// NewDesiredStateOfWorld returns a new instance of DesiredStateOfWorld.
func NewDesiredStateOfWorld(volumePluginMgr *volume.VolumePluginMgr) DesiredStateOfWorld {
	return &desiredStateOfWorld{
		nodesManaged:    make(map[string]nodeManaged),
		volumePluginMgr: volumePluginMgr,
	}
}

type desiredStateOfWorld struct {
	// nodesManaged is a map containing the set of nodes managed by the attach/
	// detach controller. The key in this map is the name of the node and the
	// value is a node object containing more information about the node.
	nodesManaged map[string]nodeManaged
	// volumePluginMgr is the volume plugin manager used to create volume
	// plugin objects.
	volumePluginMgr *volume.VolumePluginMgr
	sync.RWMutex
}

// nodeManaged represents a node that is being managed by the attach/detach
// controller.
type nodeManaged struct {
	// nodName contains the name of this node.
	nodeName string

	// volumesToAttach is a map containing the set of volumes that should be
	// attached to this node. The key in the map is the name of the volume and
	// the value is a pod object containing more information about the volume.
	volumesToAttach map[api.UniqueDeviceName]volumeToAttach
}

// The volume object represents a volume that should be attached to a node.
type volumeToAttach struct {
	// volumeName contains the unique identifier for this volume.
	volumeName api.UniqueDeviceName

	// spec is the volume spec containing the specification for this volume.
	// Used to generate the volume plugin object, and passed to attach/detach
	// methods.
	spec *volume.Spec

	// scheduledPods is a map containing the set of pods that reference this
	// volume and are scheduled to the underlying node. The key in the map is
	// the name of the pod and the value is a pod object containing more
	// information about the pod.
	scheduledPods map[string]pod
}

// The pod object represents a pod that references the underlying volume and is
// scheduled to the underlying node.
type pod struct {
	// podName contains the name of this pod.
	podName string
}

func (dsw *desiredStateOfWorld) AddNode(nodeName string) {
	dsw.Lock()
	defer dsw.Unlock()

	if _, nodeExists := dsw.nodesManaged[nodeName]; !nodeExists {
		dsw.nodesManaged[nodeName] = nodeManaged{
			nodeName:        nodeName,
			volumesToAttach: make(map[api.UniqueDeviceName]volumeToAttach),
		}
	}
}

func (dsw *desiredStateOfWorld) AddPod(
	podName string,
	volumeSpec *volume.Spec,
	nodeName string) (api.UniqueDeviceName, error) {
	dsw.Lock()
	defer dsw.Unlock()

	nodeObj, nodeExists := dsw.nodesManaged[nodeName]
	if !nodeExists {
		return "", fmt.Errorf(
			"no node with the name %q exists in the list of managed nodes",
			nodeName)
	}

	attachableVolumePlugin, err := dsw.volumePluginMgr.FindAttachablePluginBySpec(volumeSpec)
	if err != nil || attachableVolumePlugin == nil {
		return "", fmt.Errorf(
			"failed to get AttachablePlugin from volumeSpec for volume %q err=%v",
			volumeSpec.Name(),
			err)
	}

	volumeName, err := attachdetach.GetUniqueDeviceNameFromSpec(
		attachableVolumePlugin, volumeSpec)
	if err != nil {
		return "", fmt.Errorf(
			"failed to GenerateUniqueDeviceName for volumeSpec %q err=%v",
			volumeSpec.Name(),
			err)
	}

	volumeObj, volumeExists := nodeObj.volumesToAttach[volumeName]
	if !volumeExists {
		volumeObj = volumeToAttach{
			volumeName:    volumeName,
			spec:          volumeSpec,
			scheduledPods: make(map[string]pod),
		}
		dsw.nodesManaged[nodeName].volumesToAttach[volumeName] = volumeObj
	}

	if _, podExists := volumeObj.scheduledPods[podName]; !podExists {
		dsw.nodesManaged[nodeName].volumesToAttach[volumeName].scheduledPods[podName] =
			pod{
				podName: podName,
			}
	}

	return volumeName, nil
}

func (dsw *desiredStateOfWorld) DeleteNode(nodeName string) error {
	dsw.Lock()
	defer dsw.Unlock()

	nodeObj, nodeExists := dsw.nodesManaged[nodeName]
	if !nodeExists {
		return nil
	}

	if len(nodeObj.volumesToAttach) > 0 {
		return fmt.Errorf(
			"failed to delete node %q from list of nodes managed by attach/detach controller--the node still contains %v volumes in its list of volumes to attach",
			nodeName,
			len(nodeObj.volumesToAttach))
	}

	delete(
		dsw.nodesManaged,
		nodeName)
	return nil
}

func (dsw *desiredStateOfWorld) DeletePod(
	podName string,
	volumeName api.UniqueDeviceName,
	nodeName string) {
	dsw.Lock()
	defer dsw.Unlock()

	nodeObj, nodeExists := dsw.nodesManaged[nodeName]
	if !nodeExists {
		return
	}

	volumeObj, volumeExists := nodeObj.volumesToAttach[volumeName]
	if !volumeExists {
		return
	}
	if _, podExists := volumeObj.scheduledPods[podName]; !podExists {
		return
	}

	delete(
		dsw.nodesManaged[nodeName].volumesToAttach[volumeName].scheduledPods,
		podName)

	if len(volumeObj.scheduledPods) == 0 {
		delete(
			dsw.nodesManaged[nodeName].volumesToAttach,
			volumeName)
	}
}

func (dsw *desiredStateOfWorld) NodeExists(nodeName string) bool {
	dsw.RLock()
	defer dsw.RUnlock()

	_, nodeExists := dsw.nodesManaged[nodeName]
	return nodeExists
}

func (dsw *desiredStateOfWorld) VolumeExists(
	volumeName api.UniqueDeviceName, nodeName string) bool {
	dsw.RLock()
	defer dsw.RUnlock()

	nodeObj, nodeExists := dsw.nodesManaged[nodeName]
	if nodeExists {
		if _, volumeExists := nodeObj.volumesToAttach[volumeName]; volumeExists {
			return true
		}
	}

	return false
}

func (dsw *desiredStateOfWorld) GetVolumesToAttach() []VolumeToAttach {
	dsw.RLock()
	defer dsw.RUnlock()

	volumesToAttach := make([]VolumeToAttach, 0 /* len */, len(dsw.nodesManaged) /* cap */)
	for nodeName, nodeObj := range dsw.nodesManaged {
		for volumeName, volumeObj := range nodeObj.volumesToAttach {
			volumesToAttach = append(volumesToAttach, VolumeToAttach{NodeName: nodeName, VolumeName: volumeName, VolumeSpec: volumeObj.spec})
		}
	}

	return volumesToAttach
}
