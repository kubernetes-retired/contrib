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

package rkt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	appcschema "github.com/appc/spec/schema"
	appctypes "github.com/appc/spec/schema/types"
	"github.com/coreos/go-systemd/unit"
	rktapi "github.com/coreos/rkt/api/v1alpha"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/credentialprovider"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	proberesults "k8s.io/kubernetes/pkg/kubelet/prober/results"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"k8s.io/kubernetes/pkg/securitycontext"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	RktType = "rkt"

	minimumAppcVersion       = "0.7.4"
	minimumRktBinVersion     = "0.13.0"
	recommendedRktBinVersion = "0.13.0"
	minimumRktApiVersion     = "1.0.0-alpha"
	minimumSystemdVersion    = "219"

	systemdServiceDir = "/run/systemd/system"
	rktDataDir        = "/var/lib/rkt"
	rktLocalConfigDir = "/etc/rkt"

	kubernetesUnitPrefix  = "k8s"
	unitKubernetesSection = "X-Kubernetes"
	unitPodName           = "POD"
	unitRktID             = "RktID"
	unitRestartCount      = "RestartCount"

	k8sRktKubeletAnno      = "rkt.kubernetes.io/managed-by-kubelet"
	k8sRktKubeletAnnoValue = "true"
	k8sRktUIDAnno          = "rkt.kubernetes.io/uid"
	k8sRktNameAnno         = "rkt.kubernetes.io/name"
	k8sRktNamespaceAnno    = "rkt.kubernetes.io/namespace"
	//TODO: remove the creation time annotation once this is closed: https://github.com/coreos/rkt/issues/1789
	k8sRktCreationTimeAnno  = "rkt.kubernetes.io/created"
	k8sRktContainerHashAnno = "rkt.kubernetes.io/containerhash"
	k8sRktRestartCountAnno  = "rkt.kubernetes.io/restartcount"

	dockerPrefix = "docker://"

	authDir            = "auth.d"
	dockerAuthTemplate = `{"rktKind":"dockerAuth","rktVersion":"v1","registries":[%q],"credentials":{"user":%q,"password":%q}}`

	defaultImageTag          = "latest"
	defaultRktAPIServiceAddr = "localhost:15441"
	defaultNetworkName       = "rkt.kubernetes.io"
)

// Runtime implements the Containerruntime for rkt. The implementation
// uses systemd, so in order to run this runtime, systemd must be installed
// on the machine.
type Runtime struct {
	systemd systemdInterface
	// The grpc client for rkt api-service.
	apisvcConn *grpc.ClientConn
	apisvc     rktapi.PublicAPIClient
	// The absolute path to rkt binary.
	rktBinAbsPath string
	config        *Config
	// TODO(yifan): Refactor this to be generic keyring.
	dockerKeyring credentialprovider.DockerKeyring

	containerRefManager *kubecontainer.RefManager
	generator           kubecontainer.RunContainerOptionsGenerator
	recorder            record.EventRecorder
	livenessManager     proberesults.Manager
	volumeGetter        volumeGetter
	imagePuller         kubecontainer.ImagePuller

	// Versions
	binVersion     rktVersion
	apiVersion     rktVersion
	appcVersion    rktVersion
	systemdVersion systemdVersion
}

var _ kubecontainer.Runtime = &Runtime{}

// TODO(yifan): Remove this when volumeManager is moved to separate package.
type volumeGetter interface {
	GetVolumes(podUID types.UID) (kubecontainer.VolumeMap, bool)
}

// New creates the rkt container runtime which implements the container runtime interface.
// It will test if the rkt binary is in the $PATH, and whether we can get the
// version of it. If so, creates the rkt container runtime, otherwise returns an error.
func New(config *Config,
	generator kubecontainer.RunContainerOptionsGenerator,
	recorder record.EventRecorder,
	containerRefManager *kubecontainer.RefManager,
	livenessManager proberesults.Manager,
	volumeGetter volumeGetter,
	imageBackOff *util.Backoff,
	serializeImagePulls bool,
) (*Runtime, error) {
	// Create dbus connection.
	systemd, err := newSystemd()
	if err != nil {
		return nil, fmt.Errorf("rkt: cannot create systemd interface: %v", err)
	}

	// TODO(yifan): Use secure connection.
	apisvcConn, err := grpc.Dial(defaultRktAPIServiceAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("rkt: cannot connect to rkt api service: %v", err)
	}

	rktBinAbsPath := config.Path
	if rktBinAbsPath == "" {
		// No default rkt path was set, so try to find one in $PATH.
		var err error
		rktBinAbsPath, err = exec.LookPath("rkt")
		if err != nil {
			return nil, fmt.Errorf("cannot find rkt binary: %v", err)
		}
	}

	rkt := &Runtime{
		systemd:             systemd,
		rktBinAbsPath:       rktBinAbsPath,
		apisvcConn:          apisvcConn,
		apisvc:              rktapi.NewPublicAPIClient(apisvcConn),
		config:              config,
		dockerKeyring:       credentialprovider.NewDockerKeyring(),
		containerRefManager: containerRefManager,
		generator:           generator,
		recorder:            recorder,
		livenessManager:     livenessManager,
		volumeGetter:        volumeGetter,
	}
	if serializeImagePulls {
		rkt.imagePuller = kubecontainer.NewSerializedImagePuller(recorder, rkt, imageBackOff)
	} else {
		rkt.imagePuller = kubecontainer.NewImagePuller(recorder, rkt, imageBackOff)
	}

	if err := rkt.checkVersion(minimumRktBinVersion, recommendedRktBinVersion, minimumAppcVersion, minimumRktApiVersion, minimumSystemdVersion); err != nil {
		// TODO(yifan): Latest go-systemd version have the ability to close the
		// dbus connection. However the 'docker/libcontainer' package is using
		// the older go-systemd version, so we can't update the go-systemd version.
		rkt.apisvcConn.Close()
		return nil, err
	}
	return rkt, nil
}

func (r *Runtime) buildCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(r.rktBinAbsPath)
	cmd.Args = append(cmd.Args, r.config.buildGlobalOptions()...)
	cmd.Args = append(cmd.Args, args...)
	return cmd
}

// convertToACName converts a string into ACName.
func convertToACName(name string) appctypes.ACName {
	// Note that as the 'name' already matches 'DNS_LABEL'
	// defined in pkg/api/types.go, there shouldn't be error or panic.
	acname, _ := appctypes.SanitizeACName(name)
	return *appctypes.MustACName(acname)
}

// runCommand invokes rkt binary with arguments and returns the result
// from stdout in a list of strings. Each string in the list is a line.
func (r *Runtime) runCommand(args ...string) ([]string, error) {
	glog.V(4).Info("rkt: Run command:", args)

	var stdout, stderr bytes.Buffer
	cmd := r.buildCommand(args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: %v\nstdout: %v\nstderr: %v", args, err, stdout.String(), stderr.String())
	}
	return strings.Split(strings.TrimSpace(stdout.String()), "\n"), nil
}

// makePodServiceFileName constructs the unit file name for a pod using its UID.
func makePodServiceFileName(uid types.UID) string {
	// TODO(yifan): Add name for readability? We need to consider the
	// limit of the length.
	return fmt.Sprintf("%s_%s.service", kubernetesUnitPrefix, uid)
}

type resource struct {
	limit   string
	request string
}

// rawValue converts a string to *json.RawMessage
func rawValue(value string) *json.RawMessage {
	msg := json.RawMessage(value)
	return &msg
}

// rawValue converts the request, limit to *json.RawMessage
func rawRequestLimit(request, limit string) *json.RawMessage {
	if request == "" {
		request = limit
	}
	if limit == "" {
		limit = request
	}
	return rawValue(fmt.Sprintf(`{"request":%q,"limit":%q}`, request, limit))
}

// setIsolators overrides the isolators of the pod manifest if necessary.
// TODO need an apply config in security context for rkt
func setIsolators(app *appctypes.App, c *api.Container) error {
	hasCapRequests := securitycontext.HasCapabilitiesRequest(c)
	if hasCapRequests || len(c.Resources.Limits) > 0 || len(c.Resources.Requests) > 0 {
		app.Isolators = []appctypes.Isolator{}
	}

	// Retained capabilities/privileged.
	privileged := false
	if c.SecurityContext != nil && c.SecurityContext.Privileged != nil {
		privileged = *c.SecurityContext.Privileged
	}

	var addCaps string
	if privileged {
		addCaps = getAllCapabilities()
	} else {
		if hasCapRequests {
			addCaps = getCapabilities(c.SecurityContext.Capabilities.Add)
		}
	}
	if len(addCaps) > 0 {
		// TODO(yifan): Replace with constructor, see:
		// https://github.com/appc/spec/issues/268
		isolator := appctypes.Isolator{
			Name:     "os/linux/capabilities-retain-set",
			ValueRaw: rawValue(fmt.Sprintf(`{"set":[%s]}`, addCaps)),
		}
		app.Isolators = append(app.Isolators, isolator)
	}

	// Removed capabilities.
	var dropCaps string
	if hasCapRequests {
		dropCaps = getCapabilities(c.SecurityContext.Capabilities.Drop)
	}
	if len(dropCaps) > 0 {
		// TODO(yifan): Replace with constructor, see:
		// https://github.com/appc/spec/issues/268
		isolator := appctypes.Isolator{
			Name:     "os/linux/capabilities-remove-set",
			ValueRaw: rawValue(fmt.Sprintf(`{"set":[%s]}`, dropCaps)),
		}
		app.Isolators = append(app.Isolators, isolator)
	}

	// Resources.
	resources := make(map[api.ResourceName]resource)
	for name, quantity := range c.Resources.Limits {
		resources[name] = resource{limit: quantity.String()}
	}
	for name, quantity := range c.Resources.Requests {
		r, ok := resources[name]
		if !ok {
			r = resource{}
		}
		r.request = quantity.String()
		resources[name] = r
	}
	var acName appctypes.ACIdentifier
	for name, res := range resources {
		switch name {
		case api.ResourceCPU:
			acName = "resource/cpu"
		case api.ResourceMemory:
			acName = "resource/memory"
		default:
			return fmt.Errorf("resource type not supported: %v", name)
		}
		// TODO(yifan): Replace with constructor, see:
		// https://github.com/appc/spec/issues/268
		isolator := appctypes.Isolator{
			Name:     acName,
			ValueRaw: rawRequestLimit(res.request, res.limit),
		}
		app.Isolators = append(app.Isolators, isolator)
	}
	return nil
}

// mergeEnv merges the optEnv with the image's environments.
// The environments defined in the image will be overridden by
// the ones with the same name in optEnv.
func mergeEnv(app *appctypes.App, optEnv []kubecontainer.EnvVar) {
	envMap := make(map[string]string)
	for _, e := range app.Environment {
		envMap[e.Name] = e.Value
	}
	for _, e := range optEnv {
		envMap[e.Name] = e.Value
	}
	app.Environment = nil
	for name, value := range envMap {
		app.Environment = append(app.Environment, appctypes.EnvironmentVariable{
			Name:  name,
			Value: value,
		})
	}
}

// mergeMounts merges the optMounts with the image's mount points.
// The mount points defined in the image will be overridden by the ones
// with the same name in optMounts.
func mergeMounts(app *appctypes.App, optMounts []kubecontainer.Mount) {
	mountMap := make(map[appctypes.ACName]appctypes.MountPoint)
	for _, m := range app.MountPoints {
		mountMap[m.Name] = m
	}
	for _, m := range optMounts {
		mpName := convertToACName(m.Name)
		mountMap[mpName] = appctypes.MountPoint{
			Name:     mpName,
			Path:     m.ContainerPath,
			ReadOnly: m.ReadOnly,
		}
	}
	app.MountPoints = nil
	for _, mount := range mountMap {
		app.MountPoints = append(app.MountPoints, mount)
	}
}

// mergePortMappings merges the optPortMappings with the image's port mappings.
// The port mappings defined in the image will be overridden by the ones
// with the same name in optPortMappings.
func mergePortMappings(app *appctypes.App, optPortMappings []kubecontainer.PortMapping) {
	portMap := make(map[appctypes.ACName]appctypes.Port)
	for _, p := range app.Ports {
		portMap[p.Name] = p
	}
	for _, p := range optPortMappings {
		pName := convertToACName(p.Name)
		portMap[pName] = appctypes.Port{
			Name:     pName,
			Protocol: string(p.Protocol),
			Port:     uint(p.ContainerPort),
		}
	}
	app.Ports = nil
	for _, port := range portMap {
		app.Ports = append(app.Ports, port)
	}
}

// setApp overrides the app's fields if any of them are specified in the
// container's spec.
func setApp(app *appctypes.App, c *api.Container, opts *kubecontainer.RunContainerOptions) error {
	// Override the exec.

	if len(c.Command) > 0 {
		app.Exec = c.Command
	}
	if len(c.Args) > 0 {
		app.Exec = append(app.Exec, c.Args...)
	}

	// TODO(yifan): Use non-root user in the future, see:
	// https://github.com/coreos/rkt/issues/820
	app.User, app.Group = "0", "0"

	// Override the working directory.
	if len(c.WorkingDir) > 0 {
		app.WorkingDirectory = c.WorkingDir
	}

	// Notes that we don't create Mounts section in the pod manifest here,
	// as Mounts will be automatically generated by rkt.
	mergeMounts(app, opts.Mounts)
	mergeEnv(app, opts.Envs)
	mergePortMappings(app, opts.PortMappings)

	// Override isolators.
	return setIsolators(app, c)
}

// makePodManifest transforms a kubelet pod spec to the rkt pod manifest.
func (r *Runtime) makePodManifest(pod *api.Pod, pullSecrets []api.Secret) (*appcschema.PodManifest, error) {
	var globalPortMappings []kubecontainer.PortMapping
	manifest := appcschema.BlankPodManifest()

	listResp, err := r.apisvc.ListPods(context.Background(), &rktapi.ListPodsRequest{
		Detail:  true,
		Filters: kubernetesPodFilters(pod.UID),
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't list pods: %v", err)
	}

	restartCount := 0
	for _, pod := range listResp.Pods {
		manifest := &appcschema.PodManifest{}
		err = json.Unmarshal(pod.Manifest, manifest)
		if err != nil {
			glog.Warningf("rkt: error unmatshaling pod manifest: %v", err)
			continue
		}

		if countString, ok := manifest.Annotations.Get(k8sRktRestartCountAnno); ok {
			num, err := strconv.Atoi(countString)
			if err != nil {
				glog.Warningf("rkt: error reading restart count on pod: %v", err)
				continue
			}
			if num+1 > restartCount {
				restartCount = num + 1
			}
		}
	}

	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktKubeletAnno), k8sRktKubeletAnnoValue)
	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktUIDAnno), string(pod.UID))
	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktNameAnno), pod.Name)
	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktNamespaceAnno), pod.Namespace)
	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktCreationTimeAnno), strconv.FormatInt(time.Now().Unix(), 10))
	manifest.Annotations.Set(*appctypes.MustACIdentifier(k8sRktRestartCountAnno), strconv.Itoa(restartCount))

	for _, c := range pod.Spec.Containers {
		app, portMappings, err := r.newAppcRuntimeApp(pod, c, pullSecrets)
		if err != nil {
			return nil, err
		}
		manifest.Apps = append(manifest.Apps, *app)
		globalPortMappings = append(globalPortMappings, portMappings...)
	}

	volumeMap, ok := r.volumeGetter.GetVolumes(pod.UID)
	if !ok {
		return nil, fmt.Errorf("cannot get the volumes for pod %q", format.Pod(pod))
	}

	// Set global volumes.
	for vname, volume := range volumeMap {
		manifest.Volumes = append(manifest.Volumes, appctypes.Volume{
			Name:   convertToACName(vname),
			Kind:   "host",
			Source: volume.Builder.GetPath(),
		})
	}

	// Set global ports.
	for _, port := range globalPortMappings {
		manifest.Ports = append(manifest.Ports, appctypes.ExposedPort{
			Name:     convertToACName(port.Name),
			HostPort: uint(port.HostPort),
		})
	}
	// TODO(yifan): Set pod-level isolators once it's supported in kubernetes.
	return manifest, nil
}

func (r *Runtime) newAppcRuntimeApp(pod *api.Pod, c api.Container, pullSecrets []api.Secret) (*appcschema.RuntimeApp, []kubecontainer.PortMapping, error) {
	if err, _ := r.imagePuller.PullImage(pod, &c, pullSecrets); err != nil {
		return nil, nil, err
	}
	imgManifest, err := r.getImageManifest(c.Image)
	if err != nil {
		return nil, nil, err
	}

	if imgManifest.App == nil {
		imgManifest.App = new(appctypes.App)
	}

	imageID, err := r.getImageID(c.Image)
	if err != nil {
		return nil, nil, err
	}
	hash, err := appctypes.NewHash(imageID)
	if err != nil {
		return nil, nil, err
	}

	opts, err := r.generator.GenerateRunContainerOptions(pod, &c)
	if err != nil {
		return nil, nil, err
	}

	if err := setApp(imgManifest.App, &c, opts); err != nil {
		return nil, nil, err
	}

	return &appcschema.RuntimeApp{
		Name:  convertToACName(c.Name),
		Image: appcschema.RuntimeImage{ID: *hash},
		App:   imgManifest.App,
		Annotations: []appctypes.Annotation{
			{
				Name:  *appctypes.MustACIdentifier(k8sRktContainerHashAnno),
				Value: strconv.FormatUint(kubecontainer.HashContainer(&c), 10),
			},
		},
	}, opts.PortMappings, nil
}

func runningKubernetesPodFilters(uid types.UID) []*rktapi.PodFilter {
	return []*rktapi.PodFilter{
		{
			States: []rktapi.PodState{
				rktapi.PodState_POD_STATE_RUNNING,
			},
			Annotations: []*rktapi.KeyValue{
				{
					Key:   k8sRktKubeletAnno,
					Value: k8sRktKubeletAnnoValue,
				},
				{
					Key:   k8sRktUIDAnno,
					Value: string(uid),
				},
			},
		},
	}
}

func kubernetesPodFilters(uid types.UID) []*rktapi.PodFilter {
	return []*rktapi.PodFilter{
		{
			Annotations: []*rktapi.KeyValue{
				{
					Key:   k8sRktKubeletAnno,
					Value: k8sRktKubeletAnnoValue,
				},
				{
					Key:   k8sRktUIDAnno,
					Value: string(uid),
				},
			},
		},
	}
}

func newUnitOption(section, name, value string) *unit.UnitOption {
	return &unit.UnitOption{Section: section, Name: name, Value: value}
}

// apiPodToruntimePod converts an api.Pod to kubelet/container.Pod.
func apiPodToruntimePod(uuid string, pod *api.Pod) *kubecontainer.Pod {
	p := &kubecontainer.Pod{
		ID:        pod.UID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]
		p.Containers = append(p.Containers, &kubecontainer.Container{
			ID:      buildContainerID(&containerID{uuid, c.Name}),
			Name:    c.Name,
			Image:   c.Image,
			Hash:    kubecontainer.HashContainer(c),
			Created: time.Now().Unix(),
		})
	}
	return p
}

// serviceFilePath returns the absolute path of the service file.
func serviceFilePath(serviceName string) string {
	return path.Join(systemdServiceDir, serviceName)
}

// preparePod will:
//
// 1. Invoke 'rkt prepare' to prepare the pod, and get the rkt pod uuid.
// 2. Create the unit file and save it under systemdUnitDir.
//
// On success, it will return a string that represents name of the unit file
// and the runtime pod.
func (r *Runtime) preparePod(pod *api.Pod, pullSecrets []api.Secret) (string, *kubecontainer.Pod, error) {
	// Generate the pod manifest from the pod spec.
	manifest, err := r.makePodManifest(pod, pullSecrets)
	if err != nil {
		return "", nil, err
	}
	manifestFile, err := ioutil.TempFile("", fmt.Sprintf("manifest-%s-", pod.Name))
	if err != nil {
		return "", nil, err
	}
	defer func() {
		manifestFile.Close()
		if err := os.Remove(manifestFile.Name()); err != nil {
			glog.Warningf("rkt: Cannot remove temp manifest file %q: %v", manifestFile.Name(), err)
		}
	}()

	data, err := json.Marshal(manifest)
	if err != nil {
		return "", nil, err
	}
	// Since File.Write returns error if the written length is less than len(data),
	// so check error is enough for us.
	if _, err := manifestFile.Write(data); err != nil {
		return "", nil, err
	}

	// Run 'rkt prepare' to get the rkt UUID.
	cmds := []string{"prepare", "--quiet", "--pod-manifest", manifestFile.Name()}
	if r.config.Stage1Image != "" {
		cmds = append(cmds, "--stage1-image", r.config.Stage1Image)
	}
	output, err := r.runCommand(cmds...)
	if err != nil {
		return "", nil, err
	}
	if len(output) != 1 {
		return "", nil, fmt.Errorf("invalid output from 'rkt prepare': %v", output)
	}
	uuid := output[0]
	glog.V(4).Infof("'rkt prepare' returns %q", uuid)

	// Create systemd service file for the rkt pod.
	var runPrepared string
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.HostNetwork {
		runPrepared = fmt.Sprintf("%s run-prepared --mds-register=false --net=host %s", r.rktBinAbsPath, uuid)
	} else {
		runPrepared = fmt.Sprintf("%s run-prepared --mds-register=false --net=%s %s", r.rktBinAbsPath, defaultNetworkName, uuid)
	}

	// TODO handle pod.Spec.HostPID
	// TODO handle pod.Spec.HostIPC

	units := []*unit.UnitOption{
		// This makes the service show up for 'systemctl list-units' even if it exits successfully.
		newUnitOption("Service", "RemainAfterExit", "true"),
		newUnitOption("Service", "ExecStart", runPrepared),
		// This enables graceful stop.
		newUnitOption("Service", "KillMode", "mixed"),
	}

	// Check if there's old rkt pod corresponding to the same pod, if so, update the restart count.
	var needReload bool
	serviceName := makePodServiceFileName(pod.UID)
	if _, err := os.Stat(serviceFilePath(serviceName)); err == nil {
		// Service file already exists, that means the pod is being restarted.
		needReload = true
	}

	glog.V(4).Infof("rkt: Creating service file %q for pod %q", serviceName, format.Pod(pod))
	serviceFile, err := os.Create(serviceFilePath(serviceName))
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(serviceFile, unit.Serialize(units)); err != nil {
		return "", nil, err
	}
	serviceFile.Close()
	if needReload {
		if err := r.systemd.Reload(); err != nil {
			return "", nil, err
		}
	}

	return serviceName, apiPodToruntimePod(uuid, pod), nil
}

// generateEvents is a helper function that generates some container
// life cycle events for containers in a pod.
func (r *Runtime) generateEvents(runtimePod *kubecontainer.Pod, reason string, failure error) {
	// Set up container references.
	for _, c := range runtimePod.Containers {
		containerID := c.ID
		id, err := parseContainerID(containerID)
		if err != nil {
			glog.Warningf("Invalid container ID %q", containerID)
			continue
		}

		ref, ok := r.containerRefManager.GetRef(containerID)
		if !ok {
			glog.Warningf("No ref for container %q", containerID)
			continue
		}

		// Note that 'rkt id' is the pod id.
		uuid := util.ShortenString(id.uuid, 8)
		switch reason {
		case "Created":
			r.recorder.Eventf(ref, api.EventTypeNormal, kubecontainer.CreatedContainer, "Created with rkt id %v", uuid)
		case "Started":
			r.recorder.Eventf(ref, api.EventTypeNormal, kubecontainer.StartedContainer, "Started with rkt id %v", uuid)
		case "Failed":
			r.recorder.Eventf(ref, api.EventTypeWarning, kubecontainer.FailedToStartContainer, "Failed to start with rkt id %v with error %v", uuid, failure)
		case "Killing":
			r.recorder.Eventf(ref, api.EventTypeNormal, kubecontainer.KillingContainer, "Killing with rkt id %v", uuid)
		default:
			glog.Errorf("rkt: Unexpected event %q", reason)
		}
	}
	return
}

// RunPod first creates the unit file for a pod, and then
// starts the unit over d-bus.
func (r *Runtime) RunPod(pod *api.Pod, pullSecrets []api.Secret) error {
	glog.V(4).Infof("Rkt starts to run pod: name %q.", format.Pod(pod))

	name, runtimePod, prepareErr := r.preparePod(pod, pullSecrets)

	// Set container references and generate events.
	// If preparedPod fails, then send out 'failed' events for each container.
	// Otherwise, store the container references so we can use them later to send events.
	for i, c := range pod.Spec.Containers {
		ref, err := kubecontainer.GenerateContainerRef(pod, &c)
		if err != nil {
			glog.Errorf("Couldn't make a ref to pod %q, container %v: '%v'", format.Pod(pod), c.Name, err)
			continue
		}
		if prepareErr != nil {
			r.recorder.Eventf(ref, api.EventTypeWarning, kubecontainer.FailedToCreateContainer, "Failed to create rkt container with error: %v", prepareErr)
			continue
		}
		containerID := runtimePod.Containers[i].ID
		r.containerRefManager.SetRef(containerID, ref)
	}

	if prepareErr != nil {
		return prepareErr
	}

	r.generateEvents(runtimePod, "Created", nil)

	// RestartUnit has the same effect as StartUnit if the unit is not running, besides it can restart
	// a unit if the unit file is changed and reloaded.
	reschan := make(chan string)
	_, err := r.systemd.RestartUnit(name, "replace", reschan)
	if err != nil {
		r.generateEvents(runtimePod, "Failed", err)
		return err
	}

	res := <-reschan
	if res != "done" {
		err := fmt.Errorf("Failed to restart unit %q: %s", name, res)
		r.generateEvents(runtimePod, "Failed", err)
		return err
	}

	r.generateEvents(runtimePod, "Started", nil)

	return nil
}

// convertRktPod will convert a rktapi.Pod to a kubecontainer.Pod
func (r *Runtime) convertRktPod(rktpod *rktapi.Pod) (*kubecontainer.Pod, error) {
	manifest := &appcschema.PodManifest{}
	err := json.Unmarshal(rktpod.Manifest, manifest)
	if err != nil {
		return nil, err
	}

	podUID, ok := manifest.Annotations.Get(k8sRktUIDAnno)
	if !ok {
		return nil, fmt.Errorf("pod is missing annotation %s", k8sRktUIDAnno)
	}
	podName, ok := manifest.Annotations.Get(k8sRktNameAnno)
	if !ok {
		return nil, fmt.Errorf("pod is missing annotation %s", k8sRktNameAnno)
	}
	podNamespace, ok := manifest.Annotations.Get(k8sRktNamespaceAnno)
	if !ok {
		return nil, fmt.Errorf("pod is missing annotation %s", k8sRktNamespaceAnno)
	}
	podCreatedString, ok := manifest.Annotations.Get(k8sRktCreationTimeAnno)
	if !ok {
		return nil, fmt.Errorf("pod is missing annotation %s", k8sRktCreationTimeAnno)
	}
	podCreated, err := strconv.ParseInt(podCreatedString, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse pod creation timestamp: %v", err)
	}

	kubepod := &kubecontainer.Pod{
		ID:        types.UID(podUID),
		Name:      podName,
		Namespace: podNamespace,
	}

	for i, app := range rktpod.Apps {
		// The order of the apps is determined by the rkt pod manifest.
		// TODO(yifan): Let the server to unmarshal the annotations? https://github.com/coreos/rkt/issues/1872
		hashStr, ok := manifest.Apps[i].Annotations.Get(k8sRktContainerHashAnno)
		if !ok {
			return nil, fmt.Errorf("app %q is missing annotation %s", app.Name, k8sRktContainerHashAnno)
		}
		containerHash, err := strconv.ParseUint(hashStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse container's hash %q: %v", hashStr, err)
		}

		kubepod.Containers = append(kubepod.Containers, &kubecontainer.Container{
			ID:      buildContainerID(&containerID{rktpod.Id, app.Name}),
			Name:    app.Name,
			Image:   app.Image.Name,
			Hash:    containerHash,
			Created: podCreated,
			State:   appStateToContainerState(app.State),
		})
	}

	return kubepod, nil
}

// GetPods runs 'systemctl list-unit' and 'rkt list' to get the list of rkt pods.
// Then it will use the result to construct a list of container runtime pods.
// If all is false, then only running pods will be returned, otherwise all pods will be
// returned.
func (r *Runtime) GetPods(all bool) ([]*kubecontainer.Pod, error) {
	glog.V(4).Infof("Rkt getting pods")

	listReq := &rktapi.ListPodsRequest{
		Detail: true,
		Filters: []*rktapi.PodFilter{
			{
				Annotations: []*rktapi.KeyValue{
					{
						Key:   k8sRktKubeletAnno,
						Value: k8sRktKubeletAnnoValue,
					},
				},
			},
		},
	}
	if !all {
		listReq.Filters[0].States = []rktapi.PodState{rktapi.PodState_POD_STATE_RUNNING}
	}
	listResp, err := r.apisvc.ListPods(context.Background(), listReq)
	if err != nil {
		return nil, fmt.Errorf("couldn't list pods: %v", err)
	}

	var pods []*kubecontainer.Pod
	for _, pod := range listResp.Pods {
		pod, err := r.convertRktPod(pod)
		if err != nil {
			glog.Warningf("rkt: Cannot construct pod from unit file: %v.", err)
			continue
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// KillPod invokes 'systemctl kill' to kill the unit that runs the pod.
// TODO(yifan): Handle network plugin.
func (r *Runtime) KillPod(pod *api.Pod, runningPod kubecontainer.Pod) error {
	glog.V(4).Infof("Rkt is killing pod: name %q.", runningPod.Name)

	serviceName := makePodServiceFileName(runningPod.ID)
	r.generateEvents(&runningPod, "Killing", nil)
	for _, c := range runningPod.Containers {
		r.containerRefManager.ClearRef(c.ID)
	}

	// Touch the systemd service file to update the mod time so it will
	// not be garbage collected too soon.
	if err := os.Chtimes(serviceFilePath(serviceName), time.Now(), time.Now()); err != nil {
		glog.Errorf("rkt: Failed to change the modification time of the service file %q: %v", serviceName, err)
		return err
	}

	// Since all service file have 'KillMode=mixed', the processes in
	// the unit's cgroup will receive a SIGKILL if the normal stop timeouts.
	reschan := make(chan string)
	_, err := r.systemd.StopUnit(serviceName, "replace", reschan)
	if err != nil {
		glog.Errorf("rkt: Failed to stop unit %q: %v", serviceName, err)
		return err
	}

	res := <-reschan
	if res != "done" {
		glog.Errorf("rkt: Failed to stop unit %q: %s", serviceName, res)
		return err
	}

	return nil
}

// GetAPIPodStatus returns the status of the given pod.
func (r *Runtime) GetAPIPodStatus(pod *api.Pod) (*api.PodStatus, error) {
	// Get the pod status.
	podStatus, err := r.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	if err != nil {
		return nil, err
	}
	return r.ConvertPodStatusToAPIPodStatus(pod, podStatus)
}

func (r *Runtime) Type() string {
	return RktType
}

func (r *Runtime) Version() (kubecontainer.Version, error) {
	return r.binVersion, nil
}

// SyncPod syncs the running pod to match the specified desired pod.
func (r *Runtime) SyncPod(pod *api.Pod, podStatus api.PodStatus, internalPodStatus *kubecontainer.PodStatus, pullSecrets []api.Secret, backOff *util.Backoff) error {
	// TODO: (random-liu) Stop using running pod in SyncPod()
	// TODO: (random-liu) Rename podStatus to apiPodStatus, rename internalPodStatus to podStatus, and use new pod status as much as possible,
	// we may stop using apiPodStatus someday.
	runningPod := kubecontainer.ConvertPodStatusToRunningPod(internalPodStatus)
	// Add references to all containers.
	unidentifiedContainers := make(map[kubecontainer.ContainerID]*kubecontainer.Container)
	for _, c := range runningPod.Containers {
		unidentifiedContainers[c.ID] = c
	}

	restartPod := false
	for _, container := range pod.Spec.Containers {
		expectedHash := kubecontainer.HashContainer(&container)

		c := runningPod.FindContainerByName(container.Name)
		if c == nil {
			if kubecontainer.ShouldContainerBeRestartedOldVersion(&container, pod, &podStatus) {
				glog.V(3).Infof("Container %+v is dead, but RestartPolicy says that we should restart it.", container)
				// TODO(yifan): Containers in one pod are fate-sharing at this moment, see:
				// https://github.com/appc/spec/issues/276.
				restartPod = true
				break
			}
			continue
		}

		// TODO: check for non-root image directives.  See ../docker/manager.go#SyncPod

		// TODO(yifan): Take care of host network change.
		containerChanged := c.Hash != 0 && c.Hash != expectedHash
		if containerChanged {
			glog.Infof("Pod %q container %q hash changed (%d vs %d), it will be killed and re-created.", format.Pod(pod), container.Name, c.Hash, expectedHash)
			restartPod = true
			break
		}

		liveness, found := r.livenessManager.Get(c.ID)
		if found && liveness != proberesults.Success && pod.Spec.RestartPolicy != api.RestartPolicyNever {
			glog.Infof("Pod %q container %q is unhealthy, it will be killed and re-created.", format.Pod(pod), container.Name)
			restartPod = true
			break
		}

		delete(unidentifiedContainers, c.ID)
	}

	// If there is any unidentified containers, restart the pod.
	if len(unidentifiedContainers) > 0 {
		restartPod = true
	}

	if restartPod {
		// Kill the pod only if the pod is actually running.
		if len(runningPod.Containers) > 0 {
			if err := r.KillPod(pod, runningPod); err != nil {
				return err
			}
		}
		if err := r.RunPod(pod, pullSecrets); err != nil {
			return err
		}
	}
	return nil
}

// GarbageCollect collects the pods/containers.
// TODO(yifan): Enforce the gc policy, also, it would be better if we can
// just GC kubernetes pods.
func (r *Runtime) GarbageCollect(gcPolicy kubecontainer.ContainerGCPolicy) error {
	if err := exec.Command("systemctl", "reset-failed").Run(); err != nil {
		glog.Errorf("rkt: Failed to reset failed systemd services: %v, continue to gc anyway...", err)
	}

	if _, err := r.runCommand("gc", "--grace-period="+gcPolicy.MinAge.String(), "--expire-prepared="+gcPolicy.MinAge.String()); err != nil {
		glog.Errorf("rkt: Failed to gc: %v", err)
	}

	// GC all inactive systemd service files.
	units, err := r.systemd.ListUnits()
	if err != nil {
		glog.Errorf("rkt: Failed to list units: %v", err)
		return err
	}
	runningKubernetesUnits := sets.NewString()
	for _, u := range units {
		if strings.HasPrefix(u.Name, kubernetesUnitPrefix) && u.SubState == "running" {
			runningKubernetesUnits.Insert(u.Name)
		}
	}

	files, err := ioutil.ReadDir(systemdServiceDir)
	if err != nil {
		glog.Errorf("rkt: Failed to read the systemd service directory: %v", err)
		return err
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), kubernetesUnitPrefix) && !runningKubernetesUnits.Has(f.Name()) && f.ModTime().Before(time.Now().Add(-gcPolicy.MinAge)) {
			glog.V(4).Infof("rkt: Removing inactive systemd service file: %v", f.Name())
			if err := os.Remove(serviceFilePath(f.Name())); err != nil {
				glog.Warningf("rkt: Failed to remove inactive systemd service file %v: %v", f.Name(), err)
			}
		}
	}
	return nil
}

// Note: In rkt, the container ID is in the form of "UUID:appName", where
// appName is the container name.
// TODO(yifan): If the rkt is using lkvm as the stage1 image, then this function will fail.
func (r *Runtime) RunInContainer(containerID kubecontainer.ContainerID, cmd []string) ([]byte, error) {
	glog.V(4).Infof("Rkt running in container.")

	id, err := parseContainerID(containerID)
	if err != nil {
		return nil, err
	}
	args := append([]string{}, "enter", fmt.Sprintf("--app=%s", id.appName), id.uuid)
	args = append(args, cmd...)

	result, err := r.buildCommand(args...).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = &rktExitError{exitErr}
		}
	}
	return result, err
}

// rktExitError implemets /pkg/util/exec.ExitError interface.
type rktExitError struct{ *exec.ExitError }

var _ utilexec.ExitError = &rktExitError{}

func (r *rktExitError) ExitStatus() int {
	if status, ok := r.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 0
}

func (r *Runtime) AttachContainer(containerID kubecontainer.ContainerID, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool) error {
	return fmt.Errorf("unimplemented")
}

// Note: In rkt, the container ID is in the form of "UUID:appName", where UUID is
// the rkt UUID, and appName is the container name.
// TODO(yifan): If the rkt is using lkvm as the stage1 image, then this function will fail.
func (r *Runtime) ExecInContainer(containerID kubecontainer.ContainerID, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool) error {
	glog.V(4).Infof("Rkt execing in container.")

	id, err := parseContainerID(containerID)
	if err != nil {
		return err
	}
	args := append([]string{}, "enter", fmt.Sprintf("--app=%s", id.appName), id.uuid)
	args = append(args, cmd...)
	command := r.buildCommand(args...)

	if tty {
		p, err := kubecontainer.StartPty(command)
		if err != nil {
			return err
		}
		defer p.Close()

		// make sure to close the stdout stream
		defer stdout.Close()

		if stdin != nil {
			go io.Copy(p, stdin)
		}
		if stdout != nil {
			go io.Copy(stdout, p)
		}
		return command.Wait()
	}
	if stdin != nil {
		// Use an os.Pipe here as it returns true *os.File objects.
		// This way, if you run 'kubectl exec <pod> -i bash' (no tty) and type 'exit',
		// the call below to command.Run() can unblock because its Stdin is the read half
		// of the pipe.
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		go io.Copy(w, stdin)

		command.Stdin = r
	}
	if stdout != nil {
		command.Stdout = stdout
	}
	if stderr != nil {
		command.Stderr = stderr
	}
	return command.Run()
}

// PortForward executes socat in the pod's network namespace and copies
// data between stream (representing the user's local connection on their
// computer) and the specified port in the container.
//
// TODO:
//  - match cgroups of container
//  - should we support nsenter + socat on the host? (current impl)
//  - should we support nsenter + socat in a container, running with elevated privs and --pid=host?
//
// TODO(yifan): Merge with the same function in dockertools.
// TODO(yifan): If the rkt is using lkvm as the stage1 image, then this function will fail.
func (r *Runtime) PortForward(pod *kubecontainer.Pod, port uint16, stream io.ReadWriteCloser) error {
	glog.V(4).Infof("Rkt port forwarding in container.")

	listResp, err := r.apisvc.ListPods(context.Background(), &rktapi.ListPodsRequest{
		Detail:  true,
		Filters: runningKubernetesPodFilters(pod.ID),
	})
	if err != nil {
		return fmt.Errorf("couldn't list pods: %v", err)
	}

	if len(listResp.Pods) != 1 {
		var podlist []string
		for _, p := range listResp.Pods {
			podlist = append(podlist, p.Id)
		}
		return fmt.Errorf("more than one running rkt pod for the kubernetes pod [%s]", strings.Join(podlist, ", "))
	}

	socatPath, lookupErr := exec.LookPath("socat")
	if lookupErr != nil {
		return fmt.Errorf("unable to do port forwarding: socat not found.")
	}

	args := []string{"-t", fmt.Sprintf("%d", listResp.Pods[0].Pid), "-n", socatPath, "-", fmt.Sprintf("TCP4:localhost:%d", port)}

	nsenterPath, lookupErr := exec.LookPath("nsenter")
	if lookupErr != nil {
		return fmt.Errorf("unable to do port forwarding: nsenter not found.")
	}

	command := exec.Command(nsenterPath, args...)
	command.Stdout = stream

	// If we use Stdin, command.Run() won't return until the goroutine that's copying
	// from stream finishes. Unfortunately, if you have a client like telnet connected
	// via port forwarding, as long as the user's telnet client is connected to the user's
	// local listener that port forwarding sets up, the telnet session never exits. This
	// means that even if socat has finished running, command.Run() won't ever return
	// (because the client still has the connection and stream open).
	//
	// The work around is to use StdinPipe(), as Wait() (called by Run()) closes the pipe
	// when the command (socat) exits.
	inPipe, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("unable to do port forwarding: error creating stdin pipe: %v", err)
	}
	go func() {
		io.Copy(inPipe, stream)
		inPipe.Close()
	}()

	return command.Run()
}

// appStateToContainerState converts rktapi.AppState to kubecontainer.ContainerState.
func appStateToContainerState(state rktapi.AppState) kubecontainer.ContainerState {
	switch state {
	case rktapi.AppState_APP_STATE_RUNNING:
		return kubecontainer.ContainerStateRunning
	case rktapi.AppState_APP_STATE_EXITED:
		return kubecontainer.ContainerStateExited
	}
	return kubecontainer.ContainerStateUnknown
}

// getPodInfo returns the pod manifest, creation time and restart count of the pod.
func getPodInfo(pod *rktapi.Pod) (podManifest *appcschema.PodManifest, creationTime time.Time, restartCount int, err error) {
	// TODO(yifan): The manifest is only used for getting the annotations.
	// Consider to let the server to unmarshal the annotations.
	var manifest appcschema.PodManifest
	if err = json.Unmarshal(pod.Manifest, &manifest); err != nil {
		return
	}

	creationTimeStr, ok := manifest.Annotations.Get(k8sRktCreationTimeAnno)
	if !ok {
		err = fmt.Errorf("no creation timestamp in pod manifest")
		return
	}
	unixSec, err := strconv.ParseInt(creationTimeStr, 10, 64)
	if err != nil {
		return
	}

	if countString, ok := manifest.Annotations.Get(k8sRktRestartCountAnno); ok {
		restartCount, err = strconv.Atoi(countString)
		if err != nil {
			return
		}
	}

	return &manifest, time.Unix(unixSec, 0), restartCount, nil
}

// populateContainerStatus fills the container status according to the app's information.
func populateContainerStatus(pod rktapi.Pod, app rktapi.App, runtimeApp appcschema.RuntimeApp, restartCount int, creationTime time.Time) (*kubecontainer.ContainerStatus, error) {
	hashStr, ok := runtimeApp.Annotations.Get(k8sRktContainerHashAnno)
	if !ok {
		return nil, fmt.Errorf("No container hash in pod manifest")
	}

	hashNum, err := strconv.ParseUint(hashStr, 10, 64)
	if err != nil {
		return nil, err
	}

	return &kubecontainer.ContainerStatus{
		ID:    buildContainerID(&containerID{uuid: pod.Id, appName: app.Name}),
		Name:  app.Name,
		State: appStateToContainerState(app.State),
		// TODO(yifan): Use the creation/start/finished timestamp when it's implemented.
		CreatedAt: creationTime,
		StartedAt: creationTime,
		ExitCode:  int(app.ExitCode),
		Image:     app.Image.Name,
		ImageID:   "rkt://" + app.Image.Id, // TODO(yifan): Add the prefix only in api.PodStatus.
		Hash:      hashNum,
		// TODO(yifan): Note that now all apps share the same restart count, this might
		// change once apps don't share the same lifecycle.
		// See https://github.com/appc/spec/pull/547.
		RestartCount: restartCount,
		// TODO(yifan): Add reason and message field.
	}, nil
}

func (r *Runtime) GetPodStatus(uid types.UID, name, namespace string) (*kubecontainer.PodStatus, error) {
	podStatus := &kubecontainer.PodStatus{
		ID:        uid,
		Name:      name,
		Namespace: namespace,
	}

	listResp, err := r.apisvc.ListPods(context.Background(), &rktapi.ListPodsRequest{
		Detail:  true,
		Filters: kubernetesPodFilters(uid),
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't list pods: %v", err)
	}

	var latestPod *rktapi.Pod
	var latestRestartCount int = -1

	// In this loop, we group all containers from all pods together,
	// also we try to find the latest pod, so we can fill other info of the pod below.
	for _, pod := range listResp.Pods {
		manifest, creationTime, restartCount, err := getPodInfo(pod)
		if err != nil {
			glog.Warning("rkt: Couldn't get necessary info from the rkt pod, (uuid %q): %v", pod.Id, err)
			continue
		}

		if restartCount > latestRestartCount {
			latestPod = pod
			latestRestartCount = restartCount
		}

		for i, app := range pod.Apps {
			// The order of the apps is determined by the rkt pod manifest.
			// TODO(yifan): Save creationTime, restartCount in each app's annotation,
			// so we don't need to pass them.
			cs, err := populateContainerStatus(*pod, *app, manifest.Apps[i], restartCount, creationTime)
			if err != nil {
				glog.Warningf("rkt: Failed to populate container status(uuid %q, app %q): %v", pod.Id, app.Name, err)
				continue
			}
			podStatus.ContainerStatuses = append(podStatus.ContainerStatuses, cs)
		}
	}

	if latestPod != nil {
		// Try to fill the IP info.
		for _, n := range latestPod.Networks {
			if n.Name == defaultNetworkName {
				podStatus.IP = n.Ipv4
			}
		}
	}

	return podStatus, nil
}

type sortByRestartCount []*kubecontainer.ContainerStatus

func (s sortByRestartCount) Len() int           { return len(s) }
func (s sortByRestartCount) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortByRestartCount) Less(i, j int) bool { return s[i].RestartCount < s[j].RestartCount }

// TODO(yifan): Delete this function when the logic is moved to kubelet.
func (r *Runtime) ConvertPodStatusToAPIPodStatus(pod *api.Pod, status *kubecontainer.PodStatus) (*api.PodStatus, error) {
	apiPodStatus := &api.PodStatus{
		// TODO(yifan): Add reason and message field.
		PodIP: status.IP,
	}

	// Sort in the reverse order of the restart count because the
	// lastest one will have the largest restart count.
	sort.Sort(sort.Reverse(sortByRestartCount(status.ContainerStatuses)))

	containerStatuses := make(map[string]*api.ContainerStatus)
	for _, c := range status.ContainerStatuses {
		var st api.ContainerState
		switch c.State {
		case kubecontainer.ContainerStateRunning:
			st.Running = &api.ContainerStateRunning{
				StartedAt: unversioned.NewTime(c.StartedAt),
			}
		case kubecontainer.ContainerStateExited:
			if pod.Spec.RestartPolicy == api.RestartPolicyAlways ||
				pod.Spec.RestartPolicy == api.RestartPolicyOnFailure && c.ExitCode != 0 {
				// TODO(yifan): Add reason and message.
				st.Waiting = &api.ContainerStateWaiting{}
				break
			}
			st.Terminated = &api.ContainerStateTerminated{
				ExitCode:  c.ExitCode,
				StartedAt: unversioned.NewTime(c.StartedAt),
				// TODO(yifan): Add reason, message, finishedAt, signal.
				ContainerID: c.ID.String(),
			}
		default:
			// Unknown state.
			// TODO(yifan): Add reason and message.
			st.Waiting = &api.ContainerStateWaiting{}
		}

		status, ok := containerStatuses[c.Name]
		if !ok {
			containerStatuses[c.Name] = &api.ContainerStatus{
				Name:         c.Name,
				Image:        c.Image,
				ImageID:      c.ImageID,
				ContainerID:  c.ID.String(),
				RestartCount: c.RestartCount,
				State:        st,
			}
			continue
		}

		// Found multiple container statuses, fill that as last termination state.
		if status.LastTerminationState.Waiting == nil &&
			status.LastTerminationState.Running == nil &&
			status.LastTerminationState.Terminated == nil {
			status.LastTerminationState = st
		}
	}

	for _, c := range pod.Spec.Containers {
		cs, ok := containerStatuses[c.Name]
		if !ok {
			cs = &api.ContainerStatus{
				Name:  c.Name,
				Image: c.Image,
				// TODO(yifan): Add reason and message.
				State: api.ContainerState{Waiting: &api.ContainerStateWaiting{}},
			}
		}
		apiPodStatus.ContainerStatuses = append(apiPodStatus.ContainerStatuses, *cs)
	}

	return apiPodStatus, nil
}

// TODO(yifan): Delete this function when the logic is moved to kubelet.
func (r *Runtime) GetPodStatusAndAPIPodStatus(pod *api.Pod) (*kubecontainer.PodStatus, *api.PodStatus, error) {
	// Get the pod status.
	podStatus, err := r.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	if err != nil {
		return nil, nil, err
	}
	apiPodStatus, err := r.ConvertPodStatusToAPIPodStatus(pod, podStatus)
	return podStatus, apiPodStatus, err
}
