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

// Package volume implements a controller to manage volume attach and detach
// operations.
package volume

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/controller/volume/attacherdetacher"
	"k8s.io/kubernetes/pkg/controller/volume/cache"
	"k8s.io/kubernetes/pkg/controller/volume/reconciler"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/io"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util/attachdetach"
)

const (
	// loopPeriod is the ammount of time the reconciler loop waits between
	// successive executions
	reconcilerLoopPeriod time.Duration = 100 * time.Millisecond

	// reconcilerMaxWaitForUnmountDuration is the maximum amount of time the
	// attach detach controller will wait for a volume to be safely unmounted
	// from its node. Once this time has expired, the controller will assume the
	// node or kubelet are unresponsive and will detach the volume anyway.
	reconcilerMaxWaitForUnmountDuration time.Duration = 3 * time.Minute
)

// AttachDetachController defines the operations supported by this controller.
type AttachDetachController interface {
	Run(stopCh <-chan struct{})
}

// NewAttachDetachController returns a new instance of AttachDetachController.
func NewAttachDetachController(
	kubeClient internalclientset.Interface,
	podInformer framework.SharedInformer,
	nodeInformer framework.SharedInformer,
	pvcInformer framework.SharedInformer,
	pvInformer framework.SharedInformer,
	cloud cloudprovider.Interface,
	plugins []volume.VolumePlugin) (AttachDetachController, error) {
	// TODO: The default resyncPeriod for shared informers is 12 hours, this is
	// unacceptable for the attach/detach controller. For example, if a pod is
	// skipped because the node it is scheduled to didn't set its annotation in
	// time, we don't want to have to wait 12hrs before processing the pod
	// again.
	// Luckily https://github.com/kubernetes/kubernetes/issues/23394 is being
	// worked on and will split resync in to resync and relist. Once that
	// happens the resync period can be set to something much faster (30
	// seconds).
	// If that issue is not resolved in time, then this controller will have to
	// consider some unappealing alternate options: use a non-shared informer
	// and set a faster resync period even if it causes relist, or requeue
	// dropped pods so they are continuously processed until it is accepted or
	// deleted (probably can't do this with sharedInformer), etc.
	adc := &attachDetachController{
		kubeClient:  kubeClient,
		pvcInformer: pvcInformer,
		pvInformer:  pvInformer,
		cloud:       cloud,
	}

	podInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    adc.podAdd,
		UpdateFunc: adc.podUpdate,
		DeleteFunc: adc.podDelete,
	})

	nodeInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    adc.nodeAdd,
		UpdateFunc: adc.nodeUpdate,
		DeleteFunc: adc.nodeDelete,
	})

	if err := adc.volumePluginMgr.InitPlugins(plugins, adc); err != nil {
		return nil, fmt.Errorf("Could not initialize volume plugins for Attach/Detach Controller: %+v", err)
	}

	adc.desiredStateOfWorld = cache.NewDesiredStateOfWorld(&adc.volumePluginMgr)
	adc.actualStateOfWorld = cache.NewActualStateOfWorld(&adc.volumePluginMgr)
	adc.attacherDetacher = attacherdetacher.NewAttacherDetacher(&adc.volumePluginMgr)
	adc.reconciler = reconciler.NewReconciler(
		reconcilerLoopPeriod,
		reconcilerMaxWaitForUnmountDuration,
		adc.desiredStateOfWorld,
		adc.actualStateOfWorld,
		adc.attacherDetacher)

	return adc, nil
}

type attachDetachController struct {
	// kubeClient is the kube API client used by volumehost to communicate with
	// the API server.
	kubeClient internalclientset.Interface

	// pvcInformer is the shared PVC informer used to fetch and store PVC
	// objects from the API server. It is shared with other controllers and
	// therefore the PVC objects in its store should be treated as immutable.
	pvcInformer framework.SharedInformer

	// pvInformer is the shared PV informer used to fetch and store PV objects
	// from the API server. It is shared with other controllers and therefore
	// the PV objects in its store should be treated as immutable.
	pvInformer framework.SharedInformer

	// cloud provider used by volume host
	cloud cloudprovider.Interface

	// volumePluginMgr used to initialize and fetch volume plugins
	volumePluginMgr volume.VolumePluginMgr

	// desiredStateOfWorld is a data structure containing the desired state of
	// the world according to this controller: i.e. what nodes the controller
	// is managing, what volumes it wants be attached to these nodes, and which
	// pods are scheduled to those nodes referencing the volumes.
	// The data structure is populated by the controller using a stream of node
	// and pod API server objects fetched by the informers.
	desiredStateOfWorld cache.DesiredStateOfWorld

	// actualStateOfWorld is a data structure containing the actual state of
	// the world according to this controller: i.e. which volumes are attached
	// to which nodes.
	// The data structure is populated upon successful completion of attach and
	// detach actions triggered by the controller and a periodic sync with
	// storage providers for the "true" state of the world.
	actualStateOfWorld cache.ActualStateOfWorld

	// attacherDetacher is used to start asynchronous attach and operations
	attacherDetacher attacherdetacher.AttacherDetacher

	// reconciler is used to run an asynchronous periodic loop to reconcile the
	// desiredStateOfWorld with the actualStateOfWorld by triggering attach
	// detach operations using the attacherDetacher.
	reconciler reconciler.Reconciler
}

func (adc *attachDetachController) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	glog.Infof("Starting Attach Detach Controller")

	go adc.reconciler.Run(stopCh)

	<-stopCh
	glog.Infof("Shutting down Attach Detach Controller")
}

func (adc *attachDetachController) podAdd(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if pod == nil || !ok {
		return
	}

	if pod.Spec.NodeName == "" {
		// Ignore pods without NodeName, indicating they are not scheduled.
		return
	}

	adc.processPodVolumes(pod, true /* addVolumes */)
}

func (adc *attachDetachController) podUpdate(oldObj, newObj interface{}) {
	// The flow for update is the same as add.
	adc.podAdd(newObj)
}

func (adc *attachDetachController) podDelete(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if pod == nil || !ok {
		return
	}

	adc.processPodVolumes(pod, false /* addVolumes */)
}

func (adc *attachDetachController) nodeAdd(obj interface{}) {
	node, ok := obj.(*api.Node)
	if node == nil || !ok {
		return
	}

	nodeName := node.Name
	if _, exists := node.Annotations[attachdetach.ControllerManagedAnnotation]; exists {
		// Node specifies annotation indicating it should be managed by attach
		// detach controller. Add it to desired state of world.
		adc.desiredStateOfWorld.AddNode(nodeName)
	}

	adc.processVolumesInUse(nodeName, node.Status.VolumesInUse)
}

func (adc *attachDetachController) nodeUpdate(oldObj, newObj interface{}) {
	// The flow for update is the same as add.
	adc.nodeAdd(newObj)
}

func (adc *attachDetachController) nodeDelete(obj interface{}) {
	node, ok := obj.(*api.Node)
	if node == nil || !ok {
		return
	}

	nodeName := node.Name
	if err := adc.desiredStateOfWorld.DeleteNode(nodeName); err != nil {
		glog.V(10).Infof("%v", err)
	}

	adc.processVolumesInUse(nodeName, node.Status.VolumesInUse)
}

// processPodVolumes processes the volumes in the given pod and adds them to the
// desired state of the world if addVolumes is true, otherwise it removes them.
func (adc *attachDetachController) processPodVolumes(
	pod *api.Pod, addVolumes bool) {
	if pod == nil {
		return
	}

	if len(pod.Spec.Volumes) <= 0 {
		return
	}

	if !adc.desiredStateOfWorld.NodeExists(pod.Spec.NodeName) {
		// If the node the pod is scheduled to does not exist in the desired
		// state of the world data structure, that indicates the node is not
		// yet managed by the controller. Therefore, ignore the pod.
		// If the node is added to the list of managed nodes in the future,
		// future adds and updates to the pod will be processed.
		glog.V(10).Infof(
			"Skipping processing of pod %q/%q: it is scheduled to node %q which is not managed by the controller.",
			pod.Namespace,
			pod.Name,
			pod.Spec.NodeName)
		return
	}

	// Process volume spec for each volume defined in pod
	for _, podVolume := range pod.Spec.Volumes {
		volumeSpec, err := adc.createVolumeSpec(podVolume, pod.Namespace)
		if err != nil {
			glog.V(10).Infof(
				"Error processing volume %q for pod %q/%q: %v",
				podVolume.Name,
				pod.Namespace,
				pod.Name,
				err)
			continue
		}

		attachableVolumePlugin, err :=
			adc.volumePluginMgr.FindAttachablePluginBySpec(volumeSpec)
		if err != nil || attachableVolumePlugin == nil {
			glog.V(10).Infof(
				"Skipping volume %q for pod %q/%q: it does not implement attacher interface. err=%v",
				podVolume.Name,
				pod.Namespace,
				pod.Name,
				err)
			continue
		}

		if addVolumes {
			// Add volume to desired state of world
			_, err := adc.desiredStateOfWorld.AddPod(
				getUniquePodName(pod), volumeSpec, pod.Spec.NodeName)
			if err != nil {
				glog.V(10).Infof(
					"Failed to add volume %q for pod %q/%q to desiredStateOfWorld. %v",
					podVolume.Name,
					pod.Namespace,
					pod.Name,
					err)
			}

		} else {
			// Remove volume from desired state of world
			uniqueVolumeName, err := attachdetach.GetUniqueDeviceNameFromSpec(
				attachableVolumePlugin, volumeSpec)
			if err != nil {
				glog.V(10).Infof(
					"Failed to delete volume %q for pod %q/%q from desiredStateOfWorld. GenerateUniqueDeviceName failed with %v",
					podVolume.Name,
					pod.Namespace,
					pod.Name,
					err)
				continue
			}
			adc.desiredStateOfWorld.DeletePod(
				getUniquePodName(pod), uniqueVolumeName, pod.Spec.NodeName)
		}
	}

	return
}

// createVolumeSpec creates and returns a mutatable volume.Spec object for the
// specified volume. It dereference any PVC to get PV objects, if needed.
func (adc *attachDetachController) createVolumeSpec(
	podVolume api.Volume, podNamespace string) (*volume.Spec, error) {
	if pvcSource := podVolume.VolumeSource.PersistentVolumeClaim; pvcSource != nil {
		glog.V(10).Infof(
			"Found PVC, ClaimName: %q/%q",
			podNamespace,
			pvcSource.ClaimName)

		// If podVolume is a PVC, fetch the real PV behind the claim
		pvName, pvcUID, err := adc.getPVCFromCacheExtractPV(
			podNamespace, pvcSource.ClaimName)
		if err != nil {
			return nil, fmt.Errorf(
				"error processing PVC %q/%q: %v",
				podNamespace,
				pvcSource.ClaimName,
				err)
		}

		glog.V(10).Infof(
			"Found bound PV for PVC (ClaimName %q/%q pvcUID %v): pvName=%q",
			podNamespace,
			pvcSource.ClaimName,
			pvcUID,
			pvName)

		// Fetch actual PV object
		volumeSpec, err := adc.getPVSpecFromCache(
			pvName, pvcSource.ReadOnly, pvcUID)
		if err != nil {
			return nil, fmt.Errorf(
				"error processing PVC %q/%q: %v",
				podNamespace,
				pvcSource.ClaimName,
				err)
		}

		glog.V(10).Infof(
			"Extracted volumeSpec (%v) from bound PV (pvName %q) and PVC (ClaimName %q/%q pvcUID %v)",
			volumeSpec.Name,
			pvName,
			podNamespace,
			pvcSource.ClaimName,
			pvcUID)

		return volumeSpec, nil
	}

	// Do not return the original volume object, since it's from the shared
	// informer it may be mutated by another consumer.
	clonedPodVolumeObj, err := api.Scheme.DeepCopy(podVolume)
	if err != nil || clonedPodVolumeObj == nil {
		return nil, fmt.Errorf(
			"failed to deep copy %q volume object. err=%v", podVolume.Name, err)
	}

	clonedPodVolume, ok := clonedPodVolumeObj.(api.Volume)
	if !ok {
		return nil, fmt.Errorf("failed to cast clonedPodVolume %#v to api.Volume", clonedPodVolumeObj)
	}

	return volume.NewSpecFromVolume(&clonedPodVolume), nil
}

// getPVCFromCacheExtractPV fetches the PVC object with the given namespace and
// name from the shared internal PVC store extracts the name of the PV it is
// pointing to and returns it.
// This method returns an error if a PVC object does not exist in the cache
// with the given namespace/name.
// This method returns an error if the PVC object's phase is not "Bound".
func (adc *attachDetachController) getPVCFromCacheExtractPV(
	namespace string, name string) (string, types.UID, error) {
	key := name
	if len(namespace) > 0 {
		key = namespace + "/" + name
	}

	pvcObj, exists, err := adc.pvcInformer.GetStore().GetByKey(key)
	if pvcObj == nil || !exists || err != nil {
		return "", "", fmt.Errorf(
			"failed to find PVC %q in PVCInformer cache. %v",
			key,
			err)
	}

	pvc, ok := pvcObj.(*api.PersistentVolumeClaim)
	if !ok || pvc == nil {
		return "", "", fmt.Errorf(
			"failed to cast %q object %#v to PersistentVolumeClaim",
			key,
			pvcObj)
	}

	if pvc.Status.Phase != api.ClaimBound || pvc.Spec.VolumeName == "" {
		return "", "", fmt.Errorf(
			"PVC %q has non-bound phase (%q) or empty pvc.Spec.VolumeName (%q)",
			key,
			pvc.Status.Phase,
			pvc.Spec.VolumeName)
	}

	return pvc.Spec.VolumeName, pvc.UID, nil
}

// getPVSpecFromCache fetches the PV object with the given name from the shared
// internal PV store and returns a volume.Spec representing it.
// This method returns an error if a PV object does not exist in the cache with
// the given name.
// This method deep copies the PV object so the caller may use the returned
// volume.Spec object without worrying about it mutating unexpectedly.
func (adc *attachDetachController) getPVSpecFromCache(
	name string,
	pvcReadOnly bool,
	expectedClaimUID types.UID) (*volume.Spec, error) {
	pvObj, exists, err := adc.pvInformer.GetStore().GetByKey(name)
	if pvObj == nil || !exists || err != nil {
		return nil, fmt.Errorf(
			"failed to find PV %q in PVInformer cache. %v", name, err)
	}

	pv, ok := pvObj.(*api.PersistentVolume)
	if !ok || pv == nil {
		return nil, fmt.Errorf(
			"failed to cast %q object %#v to PersistentVolume", name, pvObj)
	}

	if pv.Spec.ClaimRef == nil {
		return nil, fmt.Errorf(
			"found PV object %q but it has a nil pv.Spec.ClaimRef indicating it is not yet bound to the claim",
			name)
	}

	if pv.Spec.ClaimRef.UID != expectedClaimUID {
		return nil, fmt.Errorf(
			"found PV object %q but its pv.Spec.ClaimRef.UID (%q) does not point to claim.UID (%q)",
			name,
			pv.Spec.ClaimRef.UID,
			expectedClaimUID)
	}

	// Do not return the object from the informer, since the store is shared it
	// may be mutated by another consumer.
	clonedPVObj, err := api.Scheme.DeepCopy(*pv)
	if err != nil || clonedPVObj == nil {
		return nil, fmt.Errorf(
			"failed to deep copy %q PV object. err=%v", name, err)
	}

	clonedPV, ok := clonedPVObj.(api.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf(
			"failed to cast %q clonedPV %#v to PersistentVolume", name, pvObj)
	}

	return volume.NewSpecFromPersistentVolume(&clonedPV, pvcReadOnly), nil
}

// processVolumesInUse processes the list of volumes marked as "in-use"
// according to the specified Node's Status.VolumesInUse and updates the
// corresponding volume in the actual state of the world to indicate that it is
// mounted.
func (adc *attachDetachController) processVolumesInUse(
	nodeName string, volumesInUse []api.UniqueDeviceName) {
	for _, attachedVolume := range adc.actualStateOfWorld.GetAttachedVolumesForNode(nodeName) {
		mounted := false
		for _, volumeInUse := range volumesInUse {
			if attachedVolume.VolumeName == volumeInUse {
				mounted = true
				break
			}
		}
		err := adc.actualStateOfWorld.SetVolumeMountedByNode(
			attachedVolume.VolumeName, nodeName, mounted)
		if err != nil {
			glog.Warningf(
				"SetVolumeMountedByNode(%q, %q, %q) returned an error: %v",
				attachedVolume.VolumeName, nodeName, mounted, err)
		}
	}
}

// getUniquePodName returns a unique name to reference pod by in memory caches
func getUniquePodName(pod *api.Pod) string {
	return types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}.String()
}

// VolumeHost implementation
// This is an unfortunate requirement of the current factoring of volume plugin
// initializing code. It requires kubelet specific methods used by the mounting
// code to be implemented by all initializers even if the initializer does not
// do mounting (like this attach/detach controller).
// Issue kubernetes/kubernetes/issues/14217 to fix this.
func (adc *attachDetachController) GetPluginDir(podUID string) string {
	return ""
}

func (adc *attachDetachController) GetPodVolumeDir(podUID types.UID, pluginName, volumeName string) string {
	return ""
}

func (adc *attachDetachController) GetPodPluginDir(podUID types.UID, pluginName string) string {
	return ""
}

func (adc *attachDetachController) GetKubeClient() internalclientset.Interface {
	return adc.kubeClient
}

func (adc *attachDetachController) NewWrapperMounter(volName string, spec volume.Spec, pod *api.Pod, opts volume.VolumeOptions) (volume.Mounter, error) {
	return nil, fmt.Errorf("NewWrapperMounter not supported by Attach/Detach controller's VolumeHost implementation")
}

func (adc *attachDetachController) NewWrapperUnmounter(volName string, spec volume.Spec, podUID types.UID) (volume.Unmounter, error) {
	return nil, fmt.Errorf("NewWrapperUnmounter not supported by Attach/Detach controller's VolumeHost implementation")
}

func (adc *attachDetachController) GetCloudProvider() cloudprovider.Interface {
	return adc.cloud
}

func (adc *attachDetachController) GetMounter() mount.Interface {
	return nil
}

func (adc *attachDetachController) GetWriter() io.Writer {
	return nil
}

func (adc *attachDetachController) GetHostName() string {
	return ""
}
