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

package downwardapi

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/types"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	utilstrings "k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"

	"github.com/golang/glog"
)

// ProbeVolumePlugins is the entry point for plugin detection in a package.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&downwardAPIPlugin{}}
}

const (
	downwardAPIPluginName = "kubernetes.io/downward-api"
)

// downwardAPIPlugin implements the VolumePlugin interface.
type downwardAPIPlugin struct {
	host volume.VolumeHost
}

var _ volume.VolumePlugin = &downwardAPIPlugin{}

var wrappedVolumeSpec = volume.Spec{
	Volume: &api.Volume{VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: api.StorageMediumMemory}}},
}

func (plugin *downwardAPIPlugin) Init(host volume.VolumeHost) error {
	plugin.host = host
	return nil
}

func (plugin *downwardAPIPlugin) Name() string {
	return downwardAPIPluginName
}

func (plugin *downwardAPIPlugin) CanSupport(spec *volume.Spec) bool {
	return spec.Volume != nil && spec.Volume.DownwardAPI != nil
}

func (plugin *downwardAPIPlugin) NewMounter(spec *volume.Spec, pod *api.Pod, opts volume.VolumeOptions) (volume.Mounter, error) {
	v := &downwardAPIVolume{
		volName: spec.Name(),
		items:   spec.Volume.DownwardAPI.Items,
		pod:     pod,
		podUID:  pod.UID,
		plugin:  plugin,
	}
	return &downwardAPIVolumeMounter{
		downwardAPIVolume: v,
		opts:              &opts,
	}, nil
}

func (plugin *downwardAPIPlugin) NewUnmounter(volName string, podUID types.UID) (volume.Unmounter, error) {
	return &downwardAPIVolumeUnmounter{
		&downwardAPIVolume{
			volName: volName,
			podUID:  podUID,
			plugin:  plugin,
		},
	}, nil
}

// downwardAPIVolume retrieves downward API data and placing them into the volume on the host.
type downwardAPIVolume struct {
	volName string
	items   []api.DownwardAPIVolumeFile
	pod     *api.Pod
	podUID  types.UID // TODO: remove this redundancy as soon NewUnmounter func will have *api.POD and not only types.UID
	plugin  *downwardAPIPlugin
	volume.MetricsNil
}

// downwardAPIVolumeMounter fetches info from downward API from the pod
// and dumps it in files
type downwardAPIVolumeMounter struct {
	*downwardAPIVolume
	opts *volume.VolumeOptions
}

// downwardAPIVolumeMounter implements volume.Mounter interface
var _ volume.Mounter = &downwardAPIVolumeMounter{}

// downward API volumes are always ReadOnlyManaged
func (d *downwardAPIVolume) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        true,
		Managed:         true,
		SupportsSELinux: true,
	}
}

// SetUp puts in place the volume plugin.
// This function is not idempotent by design. We want the data to be refreshed periodically.
// The internal sync interval of kubelet will drive the refresh of data.
// TODO: Add volume specific ticker and refresh loop
func (b *downwardAPIVolumeMounter) SetUp(fsGroup *int64) error {
	return b.SetUpAt(b.GetPath(), fsGroup)
}

func (b *downwardAPIVolumeMounter) SetUpAt(dir string, fsGroup *int64) error {
	glog.V(3).Infof("Setting up a downwardAPI volume %v for pod %v/%v at %v", b.volName, b.pod.Namespace, b.pod.Name, dir)
	// Wrap EmptyDir. Here we rely on the idempotency of the wrapped plugin to avoid repeatedly mounting
	wrapped, err := b.plugin.host.NewWrapperMounter(b.volName, wrappedVolumeSpec, b.pod, *b.opts)
	if err != nil {
		glog.Errorf("Couldn't setup downwardAPI volume %v for pod %v/%v: %s", b.volName, b.pod.Namespace, b.pod.Name, err.Error())
		return err
	}
	if err := wrapped.SetUpAt(dir, fsGroup); err != nil {
		glog.Errorf("Unable to setup downwardAPI volume %v for pod %v/%v: %s", b.volName, b.pod.Namespace, b.pod.Name, err.Error())
		return err
	}

	data, err := b.collectData()
	if err != nil {
		glog.Errorf("Error preparing data for downwardAPI volume %v for pod %v/%v: %s", b.volName, b.pod.Namespace, b.pod.Name, err.Error())
		return err
	}

	writerContext := fmt.Sprintf("pod %v/%v volume %v", b.pod.Namespace, b.pod.Name, b.volName)
	writer, err := volumeutil.NewAtomicWriter(dir, writerContext)
	if err != nil {
		glog.Errorf("Error creating atomic writer: %v", err)
		return err
	}

	err = writer.Write(data)
	if err != nil {
		glog.Errorf("Error writing payload to dir: %v", err)
		return err
	}

	volume.SetVolumeOwnership(b, fsGroup)

	return nil
}

// collectData collects requested downwardAPI in data map.
// Map's key is the requested name of file to dump
// Map's value is the (sorted) content of the field to be dumped in the file.
func (d *downwardAPIVolume) collectData() (map[string][]byte, error) {
	errlist := []error{}
	data := make(map[string][]byte)
	for _, fileInfo := range d.items {
		if fileInfo.FieldRef != nil {
			if values, err := fieldpath.ExtractFieldPathAsString(d.pod, fileInfo.FieldRef.FieldPath); err != nil {
				glog.Errorf("Unable to extract field %s: %s", fileInfo.FieldRef.FieldPath, err.Error())
				errlist = append(errlist, err)
			} else {
				data[path.Clean(fileInfo.Path)] = []byte(sortLines(values))
			}
		} else if fileInfo.ResourceFieldRef != nil {
			containerName := fileInfo.ResourceFieldRef.ContainerName
			if values, err := fieldpath.ExtractResourceValueByContainerName(fileInfo.ResourceFieldRef, d.pod, containerName); err != nil {
				glog.Errorf("Unable to extract field %s: %s", fileInfo.ResourceFieldRef.Resource, err.Error())
				errlist = append(errlist, err)
			} else {
				data[path.Clean(fileInfo.Path)] = []byte(sortLines(values))
			}
		}
	}
	return data, utilerrors.NewAggregate(errlist)
}

// sortLines sorts the strings generated from map based data
// (annotations and labels)
func sortLines(values string) string {
	splitted := strings.Split(values, "\n")
	sort.Strings(splitted)
	return strings.Join(splitted, "\n")
}

func (d *downwardAPIVolume) GetPath() string {
	return d.plugin.host.GetPodVolumeDir(d.podUID, utilstrings.EscapeQualifiedNameForDisk(downwardAPIPluginName), d.volName)
}

// downwardAPIVolumeCleaner handles cleaning up downwardAPI volumes
type downwardAPIVolumeUnmounter struct {
	*downwardAPIVolume
}

// downwardAPIVolumeUnmounter implements volume.Unmounter interface
var _ volume.Unmounter = &downwardAPIVolumeUnmounter{}

func (c *downwardAPIVolumeUnmounter) TearDown() error {
	return c.TearDownAt(c.GetPath())
}

func (c *downwardAPIVolumeUnmounter) TearDownAt(dir string) error {
	glog.V(3).Infof("Tearing down volume %v for pod %v at %v", c.volName, c.podUID, dir)

	// Wrap EmptyDir, let it do the teardown.
	wrapped, err := c.plugin.host.NewWrapperUnmounter(c.volName, wrappedVolumeSpec, c.podUID)
	if err != nil {
		return err
	}
	return wrapped.TearDownAt(dir)
}

func (b *downwardAPIVolumeMounter) getMetaDir() string {
	return path.Join(b.plugin.host.GetPodPluginDir(b.podUID, utilstrings.EscapeQualifiedNameForDisk(downwardAPIPluginName)), b.volName)
}
