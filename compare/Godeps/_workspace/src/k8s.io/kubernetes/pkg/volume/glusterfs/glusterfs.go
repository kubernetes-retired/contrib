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

package glusterfs

import (
	"fmt"
	"os"
	"path"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
)

// This is the primary entrypoint for volume plugins.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&glusterfsPlugin{nil, exec.New()}}
}

type glusterfsPlugin struct {
	host volume.VolumeHost
	exe  exec.Interface
}

var _ volume.VolumePlugin = &glusterfsPlugin{}
var _ volume.PersistentVolumePlugin = &glusterfsPlugin{}

const (
	glusterfsPluginName = "kubernetes.io/glusterfs"
)

func (plugin *glusterfsPlugin) Init(host volume.VolumeHost) error {
	plugin.host = host
	return nil
}

func (plugin *glusterfsPlugin) Name() string {
	return glusterfsPluginName
}

func (plugin *glusterfsPlugin) CanSupport(spec *volume.Spec) bool {
	if (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.Glusterfs == nil) ||
		(spec.Volume != nil && spec.Volume.Glusterfs == nil) {
		return false
	}
	// see if glusterfs mount helper is there
	// this needs a ls because the plugin container may be on a filesystem
	// that is not visible to the volume plugin process.
	_, err := plugin.execCommand("ls", []string{"/sbin/mount.glusterfs"})
	if err == nil {
		return true
	}

	return false

}

func (plugin *glusterfsPlugin) GetAccessModes() []api.PersistentVolumeAccessMode {
	return []api.PersistentVolumeAccessMode{
		api.ReadWriteOnce,
		api.ReadOnlyMany,
		api.ReadWriteMany,
	}
}

func (plugin *glusterfsPlugin) NewBuilder(spec *volume.Spec, pod *api.Pod, _ volume.VolumeOptions) (volume.Builder, error) {
	source, _ := plugin.getGlusterVolumeSource(spec)
	ep_name := source.EndpointsName
	ns := pod.Namespace
	ep, err := plugin.host.GetKubeClient().Endpoints(ns).Get(ep_name)
	if err != nil {
		glog.Errorf("glusterfs: failed to get endpoints %s[%v]", ep_name, err)
		return nil, err
	}
	glog.V(1).Infof("glusterfs: endpoints %v", ep)
	return plugin.newBuilderInternal(spec, ep, pod, plugin.host.GetMounter(), exec.New())
}

func (plugin *glusterfsPlugin) getGlusterVolumeSource(spec *volume.Spec) (*api.GlusterfsVolumeSource, bool) {
	// Glusterfs volumes used directly in a pod have a ReadOnly flag set by the pod author.
	// Glusterfs volumes used as a PersistentVolume gets the ReadOnly flag indirectly through the persistent-claim volume used to mount the PV
	if spec.Volume != nil && spec.Volume.Glusterfs != nil {
		return spec.Volume.Glusterfs, spec.Volume.Glusterfs.ReadOnly
	} else {
		return spec.PersistentVolume.Spec.Glusterfs, spec.ReadOnly
	}
}

func (plugin *glusterfsPlugin) newBuilderInternal(spec *volume.Spec, ep *api.Endpoints, pod *api.Pod, mounter mount.Interface, exe exec.Interface) (volume.Builder, error) {
	source, readOnly := plugin.getGlusterVolumeSource(spec)
	return &glusterfsBuilder{
		glusterfs: &glusterfs{
			volName: spec.Name(),
			mounter: mounter,
			pod:     pod,
			plugin:  plugin,
		},
		hosts:    ep,
		path:     source.Path,
		readOnly: readOnly,
		exe:      exe}, nil
}

func (plugin *glusterfsPlugin) NewCleaner(volName string, podUID types.UID) (volume.Cleaner, error) {
	return plugin.newCleanerInternal(volName, podUID, plugin.host.GetMounter())
}

func (plugin *glusterfsPlugin) newCleanerInternal(volName string, podUID types.UID, mounter mount.Interface) (volume.Cleaner, error) {
	return &glusterfsCleaner{&glusterfs{
		volName: volName,
		mounter: mounter,
		pod:     &api.Pod{ObjectMeta: api.ObjectMeta{UID: podUID}},
		plugin:  plugin,
	}}, nil
}

func (plugin *glusterfsPlugin) execCommand(command string, args []string) ([]byte, error) {
	cmd := plugin.exe.Command(command, args...)
	return cmd.CombinedOutput()
}

// Glusterfs volumes represent a bare host file or directory mount of an Glusterfs export.
type glusterfs struct {
	volName string
	pod     *api.Pod
	mounter mount.Interface
	plugin  *glusterfsPlugin
	volume.MetricsNil
}

type glusterfsBuilder struct {
	*glusterfs
	hosts    *api.Endpoints
	path     string
	readOnly bool
	exe      exec.Interface
}

var _ volume.Builder = &glusterfsBuilder{}

func (b *glusterfsBuilder) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:                    b.readOnly,
		Managed:                     false,
		SupportsOwnershipManagement: false,
		SupportsSELinux:             false,
	}
}

// SetUp attaches the disk and bind mounts to the volume path.
func (b *glusterfsBuilder) SetUp() error {
	return b.SetUpAt(b.GetPath())
}

func (b *glusterfsBuilder) SetUpAt(dir string) error {
	notMnt, err := b.mounter.IsLikelyNotMountPoint(dir)
	glog.V(4).Infof("glusterfs: mount set up: %s %v %v", dir, !notMnt, err)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !notMnt {
		return nil
	}

	os.MkdirAll(dir, 0750)
	err = b.setUpAtInternal(dir)
	if err == nil {
		return nil
	}

	// Cleanup upon failure.
	c := &glusterfsCleaner{b.glusterfs}
	c.cleanup(dir)
	return err
}

func (glusterfsVolume *glusterfs) GetPath() string {
	name := glusterfsPluginName
	return glusterfsVolume.plugin.host.GetPodVolumeDir(glusterfsVolume.pod.UID, util.EscapeQualifiedNameForDisk(name), glusterfsVolume.volName)
}

type glusterfsCleaner struct {
	*glusterfs
}

var _ volume.Cleaner = &glusterfsCleaner{}

func (c *glusterfsCleaner) TearDown() error {
	return c.TearDownAt(c.GetPath())
}

func (c *glusterfsCleaner) TearDownAt(dir string) error {
	return c.cleanup(dir)
}

func (c *glusterfsCleaner) cleanup(dir string) error {
	notMnt, err := c.mounter.IsLikelyNotMountPoint(dir)
	if err != nil {
		return fmt.Errorf("glusterfs: Error checking IsLikelyNotMountPoint: %v", err)
	}
	if notMnt {
		return os.RemoveAll(dir)
	}

	if err := c.mounter.Unmount(dir); err != nil {
		return fmt.Errorf("glusterfs: Unmounting failed: %v", err)
	}
	notMnt, mntErr := c.mounter.IsLikelyNotMountPoint(dir)
	if mntErr != nil {
		return fmt.Errorf("glusterfs: IsLikelyNotMountPoint check failed: %v", mntErr)
	}
	if notMnt {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("glusterfs: RemoveAll failed: %v", err)
		}
	}

	return nil
}

func (b *glusterfsBuilder) setUpAtInternal(dir string) error {
	var errs error

	options := []string{}
	if b.readOnly {
		options = append(options, "ro")
	}

	p := path.Join(b.glusterfs.plugin.host.GetPluginDir(glusterfsPluginName), b.glusterfs.volName)
	if err := os.MkdirAll(p, 0750); err != nil {
		return fmt.Errorf("glusterfs: mkdir failed: %v", err)
	}
	log := path.Join(p, "glusterfs.log")
	options = append(options, "log-file="+log)

	addr := make(map[string]struct{})
	for _, s := range b.hosts.Subsets {
		for _, a := range s.Addresses {
			addr[a.IP] = struct{}{}
		}
	}

	// Avoid mount storm, pick a host randomly.
	// Iterate all hosts until mount succeeds.
	for hostIP := range addr {
		errs = b.mounter.Mount(hostIP+":"+b.path, dir, "glusterfs", options)
		if errs == nil {
			return nil
		}
	}
	return fmt.Errorf("glusterfs: mount failed: %v", errs)
}
