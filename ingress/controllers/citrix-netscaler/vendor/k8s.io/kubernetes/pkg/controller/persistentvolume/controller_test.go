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

package persistentvolume

import (
	"testing"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/controller/framework"
)

// Test the real controller methods (add/update/delete claim/volume) with
// a fake API server.
// There is no controller API to 'initiate syncAll now', therefore these tests
// can't reliably simulate periodic sync of volumes/claims - it would be
// either very timing-sensitive or slow to wait for real periodic sync.
func TestControllerSync(t *testing.T) {
	expectedChanges := []int{4, 1, 1, 2, 1, 1, 1}
	tests := []controllerTest{
		// [Unit test set 5] - controller tests.
		// We test the controller as if
		// it was connected to real API server, i.e. we call add/update/delete
		// Claim/Volume methods. Also, all changes to volumes and claims are
		// sent to add/update/delete Claim/Volume as real controller would do.
		{
			// addClaim gets a new claim. Check it's bound to a volume.
			"5-2 - complete bind",
			newVolumeArray("volume5-2", "10Gi", "", "", api.VolumeAvailable, api.PersistentVolumeReclaimRetain),
			newVolumeArray("volume5-2", "10Gi", "uid5-2", "claim5-2", api.VolumeBound, api.PersistentVolumeReclaimRetain, annBoundByController),
			noclaims, /* added in testAddClaim5_2 */
			newClaimArray("claim5-2", "uid5-2", "1Gi", "volume5-2", api.ClaimBound, annBoundByController, annBindCompleted),
			noevents, noerrors,
			// Custom test function that generates an add event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				claim := newClaim("claim5-2", "uid5-2", "1Gi", "", api.ClaimPending)
				reactor.addClaimEvent(claim)
				return nil
			},
		},
		{
			// deleteClaim with a bound claim makes bound volume released.
			"5-3 - delete claim",
			newVolumeArray("volume5-3", "10Gi", "uid5-3", "claim5-3", api.VolumeBound, api.PersistentVolumeReclaimRetain, annBoundByController),
			newVolumeArray("volume5-3", "10Gi", "uid5-3", "claim5-3", api.VolumeReleased, api.PersistentVolumeReclaimRetain, annBoundByController),
			newClaimArray("claim5-3", "uid5-3", "1Gi", "volume5-3", api.ClaimBound, annBoundByController, annBindCompleted),
			noclaims,
			noevents, noerrors,
			// Custom test function that generates a delete event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				obj := ctrl.claims.List()[0]
				claim := obj.(*api.PersistentVolumeClaim)
				reactor.deleteClaimEvent(claim)
				return nil
			},
		},
		{
			// deleteVolume with a bound volume. Check the claim is Lost.
			"5-4 - delete volume",
			newVolumeArray("volume5-4", "10Gi", "uid5-4", "claim5-4", api.VolumeBound, api.PersistentVolumeReclaimRetain),
			novolumes,
			newClaimArray("claim5-4", "uid5-4", "1Gi", "volume5-4", api.ClaimBound, annBoundByController, annBindCompleted),
			newClaimArray("claim5-4", "uid5-4", "1Gi", "volume5-4", api.ClaimLost, annBoundByController, annBindCompleted),
			[]string{"Warning ClaimLost"}, noerrors,
			// Custom test function that generates a delete event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				obj := ctrl.volumes.store.List()[0]
				volume := obj.(*api.PersistentVolume)
				reactor.deleteVolumeEvent(volume)
				return nil
			},
		},
		{
			// addVolume with provisioned volume from Kubernetes 1.2. No "action"
			// is expected - it should stay bound.
			"5-5 - add bound volume from 1.2",
			novolumes,
			[]*api.PersistentVolume{addVolumeAnnotation(newVolume("volume5-5", "10Gi", "uid5-5", "claim5-5", api.VolumeBound, api.PersistentVolumeReclaimDelete), pvProvisioningRequiredAnnotationKey, pvProvisioningCompletedAnnotationValue)},
			newClaimArray("claim5-5", "uid5-5", "1Gi", "", api.ClaimPending),
			newClaimArray("claim5-5", "uid5-5", "1Gi", "volume5-5", api.ClaimBound, annBindCompleted, annBoundByController),
			noevents, noerrors,
			// Custom test function that generates a add event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				volume := newVolume("volume5-5", "10Gi", "uid5-5", "claim5-5", api.VolumeBound, api.PersistentVolumeReclaimDelete)
				volume = addVolumeAnnotation(volume, pvProvisioningRequiredAnnotationKey, pvProvisioningCompletedAnnotationValue)
				reactor.addVolumeEvent(volume)
				return nil
			},
		},
		{
			// updateVolume with provisioned volume from Kubernetes 1.2. No
			// "action" is expected - it should stay bound.
			"5-6 - update bound volume from 1.2",
			[]*api.PersistentVolume{addVolumeAnnotation(newVolume("volume5-6", "10Gi", "uid5-6", "claim5-6", api.VolumeBound, api.PersistentVolumeReclaimDelete), pvProvisioningRequiredAnnotationKey, pvProvisioningCompletedAnnotationValue)},
			[]*api.PersistentVolume{addVolumeAnnotation(newVolume("volume5-6", "10Gi", "uid5-6", "claim5-6", api.VolumeBound, api.PersistentVolumeReclaimDelete), pvProvisioningRequiredAnnotationKey, pvProvisioningCompletedAnnotationValue)},
			newClaimArray("claim5-6", "uid5-6", "1Gi", "volume5-6", api.ClaimBound),
			newClaimArray("claim5-6", "uid5-6", "1Gi", "volume5-6", api.ClaimBound, annBindCompleted),
			noevents, noerrors,
			// Custom test function that generates a add event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				volume := newVolume("volume5-6", "10Gi", "uid5-6", "claim5-6", api.VolumeBound, api.PersistentVolumeReclaimDelete)
				volume = addVolumeAnnotation(volume, pvProvisioningRequiredAnnotationKey, pvProvisioningCompletedAnnotationValue)
				reactor.modifyVolumeEvent(volume)
				return nil
			},
		},
		{
			// addVolume with unprovisioned volume from Kubernetes 1.2. The
			// volume should be deleted.
			"5-7 - add unprovisioned volume from 1.2",
			novolumes,
			novolumes,
			newClaimArray("claim5-7", "uid5-7", "1Gi", "", api.ClaimPending),
			newClaimArray("claim5-7", "uid5-7", "1Gi", "", api.ClaimPending),
			noevents, noerrors,
			// Custom test function that generates a add event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				volume := newVolume("volume5-7", "10Gi", "uid5-7", "claim5-7", api.VolumeBound, api.PersistentVolumeReclaimDelete)
				volume = addVolumeAnnotation(volume, pvProvisioningRequiredAnnotationKey, "yes")
				reactor.addVolumeEvent(volume)
				return nil
			},
		},
		{
			// updateVolume with unprovisioned volume from Kubernetes 1.2. The
			// volume should be deleted.
			"5-8 - update bound volume from 1.2",
			novolumes,
			novolumes,
			newClaimArray("claim5-8", "uid5-8", "1Gi", "", api.ClaimPending),
			newClaimArray("claim5-8", "uid5-8", "1Gi", "", api.ClaimPending),
			noevents, noerrors,
			// Custom test function that generates a add event
			func(ctrl *PersistentVolumeController, reactor *volumeReactor, test controllerTest) error {
				volume := newVolume("volume5-8", "10Gi", "uid5-8", "claim5-8", api.VolumeBound, api.PersistentVolumeReclaimDelete)
				volume = addVolumeAnnotation(volume, pvProvisioningRequiredAnnotationKey, "yes")
				reactor.modifyVolumeEvent(volume)
				return nil
			},
		},
	}

	for ix, test := range tests {
		glog.V(4).Infof("starting test %q", test.name)

		// Initialize the controller
		client := &fake.Clientset{}
		volumeSource := framework.NewFakeControllerSource()
		claimSource := framework.NewFakeControllerSource()
		ctrl := newTestController(client, volumeSource, claimSource)
		reactor := newVolumeReactor(client, ctrl, volumeSource, claimSource, test.errors)
		for _, claim := range test.initialClaims {
			claimSource.Add(claim)
			reactor.claims[claim.Name] = claim
		}
		for _, volume := range test.initialVolumes {
			volumeSource.Add(volume)
			reactor.volumes[volume.Name] = volume
		}

		// Start the controller
		count := reactor.getChangeCount()
		go ctrl.Run()

		// Wait for the controller to pass initial sync and fill its caches.
		for !ctrl.volumeController.HasSynced() ||
			!ctrl.claimController.HasSynced() ||
			len(ctrl.claims.ListKeys()) < len(test.initialClaims) ||
			len(ctrl.volumes.store.ListKeys()) < len(test.initialVolumes) {

			time.Sleep(10 * time.Millisecond)
		}
		glog.V(4).Infof("controller synced, starting test")

		// Call the tested function
		err := test.test(ctrl, reactor, test)
		if err != nil {
			t.Errorf("Test %q initial test call failed: %v", test.name, err)
		}
		// Simulate a periodic resync, just in case some events arrived in a
		// wrong order.
		ctrl.claims.Resync()
		ctrl.volumes.store.Resync()

		// Wait at least once, just in case expectedChanges[ix] == 0
		reactor.waitTest()
		// Wait for expected number of operations.
		for reactor.getChangeCount() < count+expectedChanges[ix] {
			reactor.waitTest()
		}

		ctrl.Stop()

		evaluateTestResults(ctrl, reactor, test, t)
	}
}

func storeVersion(t *testing.T, prefix string, c cache.Store, version string, expectedReturn bool) {
	pv := newVolume("pvName", "1Gi", "", "", api.VolumeAvailable, api.PersistentVolumeReclaimDelete)
	pv.ResourceVersion = version
	ret, err := storeObjectUpdate(c, pv, "volume")
	if err != nil {
		t.Errorf("%s: expected storeObjectUpdate to succeed, got: %v", prefix, err)
	}
	if expectedReturn != ret {
		t.Errorf("%s: expected storeObjectUpdate to return %v, got: %v", prefix, expectedReturn, ret)
	}

	// find the stored version

	pvObj, found, err := c.GetByKey("pvName")
	if err != nil {
		t.Errorf("expected volume 'pvName' in the cache, got error instead: %v", err)
	}
	if !found {
		t.Errorf("expected volume 'pvName' in the cache but it was not found")
	}
	pv, ok := pvObj.(*api.PersistentVolume)
	if !ok {
		t.Errorf("expected volume in the cache, got different object instead: %+v", pvObj)
	}

	if ret {
		if pv.ResourceVersion != version {
			t.Errorf("expected volume with version %s in the cache, got %s instead", version, pv.ResourceVersion)
		}
	} else {
		if pv.ResourceVersion == version {
			t.Errorf("expected volume with version other than %s in the cache, got %s instead", version, pv.ResourceVersion)
		}
	}
}

// TestControllerCache tests func storeObjectUpdate()
func TestControllerCache(t *testing.T) {
	// Cache under test
	c := cache.NewStore(framework.DeletionHandlingMetaNamespaceKeyFunc)

	// Store new PV
	storeVersion(t, "Step1", c, "1", true)
	// Store the same PV
	storeVersion(t, "Step2", c, "1", true)
	// Store newer PV
	storeVersion(t, "Step3", c, "2", true)
	// Store older PV - simulating old "PV updated" event or periodic sync with
	// old data
	storeVersion(t, "Step4", c, "1", false)
	// Store newer PV - test integer parsing ("2" > "10" as string,
	// while 2 < 10 as integers)
	storeVersion(t, "Step5", c, "10", true)
}

func TestControllerCacheParsingError(t *testing.T) {
	c := cache.NewStore(framework.DeletionHandlingMetaNamespaceKeyFunc)
	// There must be something in the cache to compare with
	storeVersion(t, "Step1", c, "1", true)

	pv := newVolume("pvName", "1Gi", "", "", api.VolumeAvailable, api.PersistentVolumeReclaimDelete)
	pv.ResourceVersion = "xxx"
	_, err := storeObjectUpdate(c, pv, "volume")
	if err == nil {
		t.Errorf("Expected parsing error, got nil instead")
	}
}

func addVolumeAnnotation(volume *api.PersistentVolume, annName, annValue string) *api.PersistentVolume {
	if volume.Annotations == nil {
		volume.Annotations = make(map[string]string)
	}
	volume.Annotations[annName] = annValue
	return volume
}
