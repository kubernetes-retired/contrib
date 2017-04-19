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

package podsecuritypolicy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	psputil "k8s.io/kubernetes/pkg/security/podsecuritypolicy/util"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

func TestCreatePodSecurityContextNonmutating(t *testing.T) {
	// Create a pod with a security context that needs filling in
	createPod := func() *api.Pod {
		return &api.Pod{
			Spec: api.PodSpec{
				SecurityContext: &api.PodSecurityContext{},
			},
		}
	}

	// Create a PSP with strategies that will populate a blank psc
	createPSP := func() *extensions.PodSecurityPolicy {
		return &extensions.PodSecurityPolicy{
			ObjectMeta: api.ObjectMeta{
				Name: "psp-sa",
			},
			Spec: extensions.PodSecurityPolicySpec{
				DefaultAddCapabilities:   []api.Capability{"foo"},
				RequiredDropCapabilities: []api.Capability{"bar"},
				RunAsUser: extensions.RunAsUserStrategyOptions{
					Rule: extensions.RunAsUserStrategyRunAsAny,
				},
				SELinux: extensions.SELinuxStrategyOptions{
					Rule: extensions.SELinuxStrategyRunAsAny,
				},
				// these are pod mutating strategies that are tested above
				FSGroup: extensions.FSGroupStrategyOptions{
					Rule: extensions.FSGroupStrategyMustRunAs,
					Ranges: []extensions.IDRange{
						{Min: 1, Max: 1},
					},
				},
				SupplementalGroups: extensions.SupplementalGroupsStrategyOptions{
					Rule: extensions.SupplementalGroupsStrategyMustRunAs,
					Ranges: []extensions.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		}
	}

	pod := createPod()
	psp := createPSP()

	provider, err := NewSimpleProvider(psp, "namespace", NewSimpleStrategyFactory())
	if err != nil {
		t.Fatalf("unable to create provider %v", err)
	}
	sc, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		t.Fatalf("unable to create psc %v", err)
	}

	// The generated security context should have filled in missing options, so they should differ
	if reflect.DeepEqual(sc, &pod.Spec.SecurityContext) {
		t.Error("expected created security context to be different than container's, but they were identical")
	}

	// Creating the provider or the security context should not have mutated the psp or pod
	if !reflect.DeepEqual(createPod(), pod) {
		diffs := diff.ObjectDiff(createPod(), pod)
		t.Errorf("pod was mutated by CreatePodSecurityContext. diff:\n%s", diffs)
	}
	if !reflect.DeepEqual(createPSP(), psp) {
		t.Error("psp was mutated by CreatePodSecurityContext")
	}
}

func TestCreateContainerSecurityContextNonmutating(t *testing.T) {
	// Create a pod with a security context that needs filling in
	createPod := func() *api.Pod {
		return &api.Pod{
			Spec: api.PodSpec{
				Containers: []api.Container{{
					SecurityContext: &api.SecurityContext{},
				}},
			},
		}
	}

	// Create a PSP with strategies that will populate a blank security context
	createPSP := func() *extensions.PodSecurityPolicy {
		var uid int64 = 1
		return &extensions.PodSecurityPolicy{
			ObjectMeta: api.ObjectMeta{
				Name: "psp-sa",
			},
			Spec: extensions.PodSecurityPolicySpec{
				DefaultAddCapabilities:   []api.Capability{"foo"},
				RequiredDropCapabilities: []api.Capability{"bar"},
				RunAsUser: extensions.RunAsUserStrategyOptions{
					Rule:   extensions.RunAsUserStrategyMustRunAs,
					Ranges: []extensions.IDRange{{Min: uid, Max: uid}},
				},
				SELinux: extensions.SELinuxStrategyOptions{
					Rule:           extensions.SELinuxStrategyMustRunAs,
					SELinuxOptions: &api.SELinuxOptions{User: "you"},
				},
				// these are pod mutating strategies that are tested above
				FSGroup: extensions.FSGroupStrategyOptions{
					Rule: extensions.FSGroupStrategyRunAsAny,
				},
				SupplementalGroups: extensions.SupplementalGroupsStrategyOptions{
					Rule: extensions.SupplementalGroupsStrategyRunAsAny,
				},
				// mutates the container SC by defaulting to true if container sets nil
				ReadOnlyRootFilesystem: true,
			},
		}
	}

	pod := createPod()
	psp := createPSP()

	provider, err := NewSimpleProvider(psp, "namespace", NewSimpleStrategyFactory())
	if err != nil {
		t.Fatalf("unable to create provider %v", err)
	}
	sc, err := provider.CreateContainerSecurityContext(pod, &pod.Spec.Containers[0])
	if err != nil {
		t.Fatalf("unable to create container security context %v", err)
	}

	// The generated security context should have filled in missing options, so they should differ
	if reflect.DeepEqual(sc, &pod.Spec.Containers[0].SecurityContext) {
		t.Error("expected created security context to be different than container's, but they were identical")
	}

	// Creating the provider or the security context should not have mutated the psp or pod
	if !reflect.DeepEqual(createPod(), pod) {
		diffs := diff.ObjectDiff(createPod(), pod)
		t.Errorf("pod was mutated by CreateContainerSecurityContext. diff:\n%s", diffs)
	}
	if !reflect.DeepEqual(createPSP(), psp) {
		t.Error("psp was mutated by CreateContainerSecurityContext")
	}
}

func TestValidatePodSecurityContextFailures(t *testing.T) {
	failHostNetworkPod := defaultPod()
	failHostNetworkPod.Spec.SecurityContext.HostNetwork = true

	failHostPIDPod := defaultPod()
	failHostPIDPod.Spec.SecurityContext.HostPID = true

	failHostIPCPod := defaultPod()
	failHostIPCPod.Spec.SecurityContext.HostIPC = true

	failSupplementalGroupPod := defaultPod()
	failSupplementalGroupPod.Spec.SecurityContext.SupplementalGroups = []int64{999}
	failSupplementalGroupPSP := defaultPSP()
	failSupplementalGroupPSP.Spec.SupplementalGroups = extensions.SupplementalGroupsStrategyOptions{
		Rule: extensions.SupplementalGroupsStrategyMustRunAs,
		Ranges: []extensions.IDRange{
			{Min: 1, Max: 1},
		},
	}

	failFSGroupPod := defaultPod()
	fsGroup := int64(999)
	failFSGroupPod.Spec.SecurityContext.FSGroup = &fsGroup
	failFSGroupPSP := defaultPSP()
	failFSGroupPSP.Spec.FSGroup = extensions.FSGroupStrategyOptions{
		Rule: extensions.FSGroupStrategyMustRunAs,
		Ranges: []extensions.IDRange{
			{Min: 1, Max: 1},
		},
	}

	failNilSELinuxPod := defaultPod()
	failSELinuxPSP := defaultPSP()
	failSELinuxPSP.Spec.SELinux.Rule = extensions.SELinuxStrategyMustRunAs
	failSELinuxPSP.Spec.SELinux.SELinuxOptions = &api.SELinuxOptions{
		Level: "foo",
	}

	failInvalidSELinuxPod := defaultPod()
	failInvalidSELinuxPod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "bar",
	}

	errorCases := map[string]struct {
		pod           *api.Pod
		psp           *extensions.PodSecurityPolicy
		expectedError string
	}{
		"failHostNetwork": {
			pod:           failHostNetworkPod,
			psp:           defaultPSP(),
			expectedError: "Host network is not allowed to be used",
		},
		"failHostPID": {
			pod:           failHostPIDPod,
			psp:           defaultPSP(),
			expectedError: "Host PID is not allowed to be used",
		},
		"failHostIPC": {
			pod:           failHostIPCPod,
			psp:           defaultPSP(),
			expectedError: "Host IPC is not allowed to be used",
		},
		"failSupplementalGroupOutOfRange": {
			pod:           failSupplementalGroupPod,
			psp:           failSupplementalGroupPSP,
			expectedError: "999 is not an allowed group",
		},
		"failSupplementalGroupEmpty": {
			pod:           defaultPod(),
			psp:           failSupplementalGroupPSP,
			expectedError: "unable to validate empty groups against required ranges",
		},
		"failFSGroupOutOfRange": {
			pod:           failFSGroupPod,
			psp:           failFSGroupPSP,
			expectedError: "999 is not an allowed group",
		},
		"failFSGroupEmpty": {
			pod:           defaultPod(),
			psp:           failFSGroupPSP,
			expectedError: "unable to validate empty groups against required ranges",
		},
		"failNilSELinux": {
			pod:           failNilSELinuxPod,
			psp:           failSELinuxPSP,
			expectedError: "unable to validate nil seLinuxOptions",
		},
		"failInvalidSELinux": {
			pod:           failInvalidSELinuxPod,
			psp:           failSELinuxPSP,
			expectedError: "does not match required level.  Found bar, wanted foo",
		},
	}
	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Fatalf("unable to create provider %v", err)
		}
		errs := provider.ValidatePodSecurityContext(v.pod, field.NewPath(""))
		if len(errs) == 0 {
			t.Errorf("%s expected validation failure but did not receive errors", k)
			continue
		}
		if !strings.Contains(errs[0].Error(), v.expectedError) {
			t.Errorf("%s received unexpected error %v", k, errs)
		}
	}
}

func TestValidateContainerSecurityContextFailures(t *testing.T) {
	// fail user strat
	failUserPSP := defaultPSP()
	var uid int64 = 999
	var badUID int64 = 1
	failUserPSP.Spec.RunAsUser = extensions.RunAsUserStrategyOptions{
		Rule:   extensions.RunAsUserStrategyMustRunAs,
		Ranges: []extensions.IDRange{{Min: uid, Max: uid}},
	}
	failUserPod := defaultPod()
	failUserPod.Spec.Containers[0].SecurityContext.RunAsUser = &badUID

	// fail selinux strat
	failSELinuxPSP := defaultPSP()
	failSELinuxPSP.Spec.SELinux = extensions.SELinuxStrategyOptions{
		Rule: extensions.SELinuxStrategyMustRunAs,
		SELinuxOptions: &api.SELinuxOptions{
			Level: "foo",
		},
	}
	failSELinuxPod := defaultPod()
	failSELinuxPod.Spec.Containers[0].SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "bar",
	}

	failPrivPod := defaultPod()
	var priv bool = true
	failPrivPod.Spec.Containers[0].SecurityContext.Privileged = &priv

	failCapsPod := defaultPod()
	failCapsPod.Spec.Containers[0].SecurityContext.Capabilities = &api.Capabilities{
		Add: []api.Capability{"foo"},
	}

	failHostDirPod := defaultPod()
	failHostDirPod.Spec.Volumes = []api.Volume{
		{
			Name: "bad volume",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{},
			},
		},
	}

	failHostPortPod := defaultPod()
	failHostPortPod.Spec.Containers[0].Ports = []api.ContainerPort{{HostPort: 1}}

	readOnlyRootFSPSP := defaultPSP()
	readOnlyRootFSPSP.Spec.ReadOnlyRootFilesystem = true

	readOnlyRootFSPodFalse := defaultPod()
	readOnlyRootFS := false
	readOnlyRootFSPodFalse.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = &readOnlyRootFS

	errorCases := map[string]struct {
		pod           *api.Pod
		psp           *extensions.PodSecurityPolicy
		expectedError string
	}{
		"failUserPSP": {
			pod:           failUserPod,
			psp:           failUserPSP,
			expectedError: "does not match required range",
		},
		"failSELinuxPSP": {
			pod:           failSELinuxPod,
			psp:           failSELinuxPSP,
			expectedError: "does not match required level",
		},
		"failPrivPSP": {
			pod:           failPrivPod,
			psp:           defaultPSP(),
			expectedError: "Privileged containers are not allowed",
		},
		"failCapsPSP": {
			pod:           failCapsPod,
			psp:           defaultPSP(),
			expectedError: "capability may not be added",
		},
		"failHostDirPSP": {
			pod:           failHostDirPod,
			psp:           defaultPSP(),
			expectedError: "hostPath volumes are not allowed to be used",
		},
		"failHostPortPSP": {
			pod:           failHostPortPod,
			psp:           defaultPSP(),
			expectedError: "Host port 1 is not allowed to be used.  Allowed ports: []",
		},
		"failReadOnlyRootFS - nil": {
			pod:           defaultPod(),
			psp:           readOnlyRootFSPSP,
			expectedError: "ReadOnlyRootFilesystem may not be nil and must be set to true",
		},
		"failReadOnlyRootFS - false": {
			pod:           readOnlyRootFSPodFalse,
			psp:           readOnlyRootFSPSP,
			expectedError: "ReadOnlyRootFilesystem must be set to true",
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Fatalf("unable to create provider %v", err)
		}
		errs := provider.ValidateContainerSecurityContext(v.pod, &v.pod.Spec.Containers[0], field.NewPath(""))
		if len(errs) == 0 {
			t.Errorf("%s expected validation failure but did not receive errors", k)
			continue
		}
		if !strings.Contains(errs[0].Error(), v.expectedError) {
			t.Errorf("%s received unexpected error %v", k, errs)
		}
	}
}

func TestValidatePodSecurityContextSuccess(t *testing.T) {
	hostNetworkPSP := defaultPSP()
	hostNetworkPSP.Spec.HostNetwork = true
	hostNetworkPod := defaultPod()
	hostNetworkPod.Spec.SecurityContext.HostNetwork = true

	hostPIDPSP := defaultPSP()
	hostPIDPSP.Spec.HostPID = true
	hostPIDPod := defaultPod()
	hostPIDPod.Spec.SecurityContext.HostPID = true

	hostIPCPSP := defaultPSP()
	hostIPCPSP.Spec.HostIPC = true
	hostIPCPod := defaultPod()
	hostIPCPod.Spec.SecurityContext.HostIPC = true

	supGroupPSP := defaultPSP()
	supGroupPSP.Spec.SupplementalGroups = extensions.SupplementalGroupsStrategyOptions{
		Rule: extensions.SupplementalGroupsStrategyMustRunAs,
		Ranges: []extensions.IDRange{
			{Min: 1, Max: 5},
		},
	}
	supGroupPod := defaultPod()
	supGroupPod.Spec.SecurityContext.SupplementalGroups = []int64{3}

	fsGroupPSP := defaultPSP()
	fsGroupPSP.Spec.FSGroup = extensions.FSGroupStrategyOptions{
		Rule: extensions.FSGroupStrategyMustRunAs,
		Ranges: []extensions.IDRange{
			{Min: 1, Max: 5},
		},
	}
	fsGroupPod := defaultPod()
	fsGroup := int64(3)
	fsGroupPod.Spec.SecurityContext.FSGroup = &fsGroup

	seLinuxPod := defaultPod()
	seLinuxPod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		User:  "user",
		Role:  "role",
		Type:  "type",
		Level: "level",
	}
	seLinuxPSP := defaultPSP()
	seLinuxPSP.Spec.SELinux.Rule = extensions.SELinuxStrategyMustRunAs
	seLinuxPSP.Spec.SELinux.SELinuxOptions = &api.SELinuxOptions{
		User:  "user",
		Role:  "role",
		Type:  "type",
		Level: "level",
	}

	errorCases := map[string]struct {
		pod *api.Pod
		psp *extensions.PodSecurityPolicy
	}{
		"pass hostNetwork validating PSP": {
			pod: hostNetworkPod,
			psp: hostNetworkPSP,
		},
		"pass hostPID validating PSP": {
			pod: hostPIDPod,
			psp: hostPIDPSP,
		},
		"pass hostIPC validating PSP": {
			pod: hostIPCPod,
			psp: hostIPCPSP,
		},
		"pass supplemental group validating PSP": {
			pod: supGroupPod,
			psp: supGroupPSP,
		},
		"pass fs group validating PSP": {
			pod: fsGroupPod,
			psp: fsGroupPSP,
		},
		"pass selinux validating PSP": {
			pod: seLinuxPod,
			psp: seLinuxPSP,
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Fatalf("unable to create provider %v", err)
		}
		errs := provider.ValidatePodSecurityContext(v.pod, field.NewPath(""))
		if len(errs) != 0 {
			t.Errorf("%s expected validation pass but received errors %v", k, errs)
			continue
		}
	}
}

func TestValidateContainerSecurityContextSuccess(t *testing.T) {
	var notPriv bool = false
	defaultPod := func() *api.Pod {
		return &api.Pod{
			Spec: api.PodSpec{
				SecurityContext: &api.PodSecurityContext{},
				Containers: []api.Container{
					{
						SecurityContext: &api.SecurityContext{
							// expected to be set by defaulting mechanisms
							Privileged: &notPriv,
							// fill in the rest for test cases
						},
					},
				},
			},
		}
	}

	// fail user strat
	userPSP := defaultPSP()
	var uid int64 = 999
	userPSP.Spec.RunAsUser = extensions.RunAsUserStrategyOptions{
		Rule:   extensions.RunAsUserStrategyMustRunAs,
		Ranges: []extensions.IDRange{{Min: uid, Max: uid}},
	}
	userPod := defaultPod()
	userPod.Spec.Containers[0].SecurityContext.RunAsUser = &uid

	// fail selinux strat
	seLinuxPSP := defaultPSP()
	seLinuxPSP.Spec.SELinux = extensions.SELinuxStrategyOptions{
		Rule: extensions.SELinuxStrategyMustRunAs,
		SELinuxOptions: &api.SELinuxOptions{
			Level: "foo",
		},
	}
	seLinuxPod := defaultPod()
	seLinuxPod.Spec.Containers[0].SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "foo",
	}

	privPSP := defaultPSP()
	privPSP.Spec.Privileged = true
	privPod := defaultPod()
	var priv bool = true
	privPod.Spec.Containers[0].SecurityContext.Privileged = &priv

	capsPSP := defaultPSP()
	capsPSP.Spec.AllowedCapabilities = []api.Capability{"foo"}
	capsPod := defaultPod()
	capsPod.Spec.Containers[0].SecurityContext.Capabilities = &api.Capabilities{
		Add: []api.Capability{"foo"},
	}

	// pod should be able to request caps that are in the required set even if not specified in the allowed set
	requiredCapsPSP := defaultPSP()
	requiredCapsPSP.Spec.DefaultAddCapabilities = []api.Capability{"foo"}
	requiredCapsPod := defaultPod()
	requiredCapsPod.Spec.Containers[0].SecurityContext.Capabilities = &api.Capabilities{
		Add: []api.Capability{"foo"},
	}

	hostDirPSP := defaultPSP()
	hostDirPSP.Spec.Volumes = []extensions.FSType{extensions.HostPath}
	hostDirPod := defaultPod()
	hostDirPod.Spec.Volumes = []api.Volume{
		{
			Name: "bad volume",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{},
			},
		},
	}

	hostPortPSP := defaultPSP()
	hostPortPSP.Spec.HostPorts = []extensions.HostPortRange{{Min: 1, Max: 1}}
	hostPortPod := defaultPod()
	hostPortPod.Spec.Containers[0].Ports = []api.ContainerPort{{HostPort: 1}}

	readOnlyRootFSPodFalse := defaultPod()
	readOnlyRootFSFalse := false
	readOnlyRootFSPodFalse.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = &readOnlyRootFSFalse

	readOnlyRootFSPodTrue := defaultPod()
	readOnlyRootFSTrue := true
	readOnlyRootFSPodTrue.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = &readOnlyRootFSTrue

	errorCases := map[string]struct {
		pod *api.Pod
		psp *extensions.PodSecurityPolicy
	}{
		"pass user must run as PSP": {
			pod: userPod,
			psp: userPSP,
		},
		"pass seLinux must run as PSP": {
			pod: seLinuxPod,
			psp: seLinuxPSP,
		},
		"pass priv validating PSP": {
			pod: privPod,
			psp: privPSP,
		},
		"pass allowed caps validating PSP": {
			pod: capsPod,
			psp: capsPSP,
		},
		"pass required caps validating PSP": {
			pod: requiredCapsPod,
			psp: requiredCapsPSP,
		},
		"pass hostDir validating PSP": {
			pod: hostDirPod,
			psp: hostDirPSP,
		},
		"pass hostPort validating PSP": {
			pod: hostPortPod,
			psp: hostPortPSP,
		},
		"pass read only root fs - nil": {
			pod: defaultPod(),
			psp: defaultPSP(),
		},
		"pass read only root fs - false": {
			pod: readOnlyRootFSPodFalse,
			psp: defaultPSP(),
		},
		"pass read only root fs - true": {
			pod: readOnlyRootFSPodTrue,
			psp: defaultPSP(),
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Fatalf("unable to create provider %v", err)
		}
		errs := provider.ValidateContainerSecurityContext(v.pod, &v.pod.Spec.Containers[0], field.NewPath(""))
		if len(errs) != 0 {
			t.Errorf("%s expected validation pass but received errors %v", k, errs)
			continue
		}
	}
}

func TestGenerateContainerSecurityContextReadOnlyRootFS(t *testing.T) {
	truePSP := defaultPSP()
	truePSP.Spec.ReadOnlyRootFilesystem = true

	trueVal := true
	expectTrue := &trueVal
	falseVal := false
	expectFalse := &falseVal

	falsePod := defaultPod()
	falsePod.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = expectFalse

	truePod := defaultPod()
	truePod.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = expectTrue

	tests := map[string]struct {
		pod      *api.Pod
		psp      *extensions.PodSecurityPolicy
		expected *bool
	}{
		"false psp, nil sc": {
			psp:      defaultPSP(),
			pod:      defaultPod(),
			expected: nil,
		},
		"false psp, false sc": {
			psp:      defaultPSP(),
			pod:      falsePod,
			expected: expectFalse,
		},
		"false psp, true sc": {
			psp:      defaultPSP(),
			pod:      truePod,
			expected: expectTrue,
		},
		"true psp, nil sc": {
			psp:      truePSP,
			pod:      defaultPod(),
			expected: expectTrue,
		},
		"true psp, false sc": {
			psp: truePSP,
			pod: falsePod,
			// expect false even though it defaults to true to ensure it doesn't change set values
			// validation catches the mismatch, not generation
			expected: expectFalse,
		},
		"true psp, true sc": {
			psp:      truePSP,
			pod:      truePod,
			expected: expectTrue,
		},
	}

	for k, v := range tests {
		provider, err := NewSimpleProvider(v.psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Errorf("%s unable to create provider %v", k, err)
			continue
		}
		sc, err := provider.CreateContainerSecurityContext(v.pod, &v.pod.Spec.Containers[0])
		if err != nil {
			t.Errorf("%s unable to create container security context %v", k, err)
			continue
		}

		if v.expected == nil && sc.ReadOnlyRootFilesystem != nil {
			t.Errorf("%s expected a nil ReadOnlyRootFilesystem but got %t", k, *sc.ReadOnlyRootFilesystem)
		}
		if v.expected != nil && sc.ReadOnlyRootFilesystem == nil {
			t.Errorf("%s expected a non nil ReadOnlyRootFilesystem but recieved nil", k)
		}
		if v.expected != nil && sc.ReadOnlyRootFilesystem != nil && (*v.expected != *sc.ReadOnlyRootFilesystem) {
			t.Errorf("%s expected a non nil ReadOnlyRootFilesystem set to %t but got %t", k, *v.expected, *sc.ReadOnlyRootFilesystem)
		}

	}
}

func defaultPSP() *extensions.PodSecurityPolicy {
	return &extensions.PodSecurityPolicy{
		ObjectMeta: api.ObjectMeta{
			Name: "psp-sa",
		},
		Spec: extensions.PodSecurityPolicySpec{
			RunAsUser: extensions.RunAsUserStrategyOptions{
				Rule: extensions.RunAsUserStrategyRunAsAny,
			},
			SELinux: extensions.SELinuxStrategyOptions{
				Rule: extensions.SELinuxStrategyRunAsAny,
			},
			FSGroup: extensions.FSGroupStrategyOptions{
				Rule: extensions.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: extensions.SupplementalGroupsStrategyOptions{
				Rule: extensions.SupplementalGroupsStrategyRunAsAny,
			},
		},
	}
}

func defaultPod() *api.Pod {
	var notPriv bool = false
	return &api.Pod{
		Spec: api.PodSpec{
			SecurityContext: &api.PodSecurityContext{
			// fill in for test cases
			},
			Containers: []api.Container{
				{
					SecurityContext: &api.SecurityContext{
						// expected to be set by defaulting mechanisms
						Privileged: &notPriv,
						// fill in the rest for test cases
					},
				},
			},
		},
	}
}

// TestValidateAllowedVolumes will test that for every field of VolumeSource we can create
// a pod with that type of volume and deny it, accept it explicitly, or accept it with
// the FSTypeAll wildcard.
func TestValidateAllowedVolumes(t *testing.T) {
	val := reflect.ValueOf(api.VolumeSource{})

	for i := 0; i < val.NumField(); i++ {
		// reflectively create the volume source
		fieldVal := val.Type().Field(i)

		volumeSource := api.VolumeSource{}
		volumeSourceVolume := reflect.New(fieldVal.Type.Elem())

		reflect.ValueOf(&volumeSource).Elem().FieldByName(fieldVal.Name).Set(volumeSourceVolume)
		volume := api.Volume{VolumeSource: volumeSource}

		// sanity check before moving on
		fsType, err := psputil.GetVolumeFSType(volume)
		if err != nil {
			t.Errorf("error getting FSType for %s: %s", fieldVal.Name, err.Error())
			continue
		}

		// add the volume to the pod
		pod := defaultPod()
		pod.Spec.Volumes = []api.Volume{volume}

		// create a PSP that allows no volumes
		psp := defaultPSP()

		provider, err := NewSimpleProvider(psp, "namespace", NewSimpleStrategyFactory())
		if err != nil {
			t.Errorf("error creating provider for %s: %s", fieldVal.Name, err.Error())
			continue
		}

		// expect a denial for this PSP and test the error message to ensure it's related to the volumesource
		errs := provider.ValidateContainerSecurityContext(pod, &pod.Spec.Containers[0], field.NewPath(""))
		if len(errs) != 1 {
			t.Errorf("expected exactly 1 error for %s but got %v", fieldVal.Name, errs)
		} else {
			if !strings.Contains(errs.ToAggregate().Error(), fmt.Sprintf("%s volumes are not allowed to be used", fsType)) {
				t.Errorf("did not find the expected error, received: %v", errs)
			}
		}

		// now add the fstype directly to the psp and it should validate
		psp.Spec.Volumes = []extensions.FSType{fsType}
		errs = provider.ValidateContainerSecurityContext(pod, &pod.Spec.Containers[0], field.NewPath(""))
		if len(errs) != 0 {
			t.Errorf("directly allowing volume expected no errors for %s but got %v", fieldVal.Name, errs)
		}

		// now change the psp to allow any volumes and the pod should still validate
		psp.Spec.Volumes = []extensions.FSType{extensions.All}
		errs = provider.ValidateContainerSecurityContext(pod, &pod.Spec.Containers[0], field.NewPath(""))
		if len(errs) != 0 {
			t.Errorf("wildcard volume expected no errors for %s but got %v", fieldVal.Name, errs)
		}
	}
}
