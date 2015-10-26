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

package securitycontextconstraints

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
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

	// Create an SCC with strategies that will populate a blank psc
	createSCC := func() *api.SecurityContextConstraints {
		return &api.SecurityContextConstraints{
			ObjectMeta: api.ObjectMeta{
				Name: "scc-sa",
			},
			RunAsUser: api.RunAsUserStrategyOptions{
				Type: api.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: api.SELinuxContextStrategyOptions{
				Type:           api.SELinuxStrategyRunAsAny,
			},
			// these are pod mutating strategies that are tested above
			FSGroup: api.FSGroupStrategyOptions{
				Type: api.FSGroupStrategyMustRunAs,
				Ranges: []api.IDRange{
					{Min: 1, Max: 1},
				},
			},
			SupplementalGroups: api.SupplementalGroupsStrategyOptions{
				Type: api.SupplementalGroupsStrategyMustRunAs,
				Ranges: []api.IDRange{
					{Min: 1, Max: 1},
				},
			},
		}
	}

	pod := createPod()
	scc := createSCC()

	provider, err := NewSimpleProvider(scc)
	if err != nil {
		t.Fatal("unable to create provider %v", err)
	}
	sc, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		t.Fatal("unable to create psc %v", err)
	}

	// The generated security context should have filled in missing options, so they should differ
	if reflect.DeepEqual(sc, &pod.Spec.SecurityContext) {
		t.Error("expected created security context to be different than container's, but they were identical")
	}

	// Creating the provider or the security context should not have mutated the scc or pod
	if !reflect.DeepEqual(createPod(), pod) {
		diff := util.ObjectDiff(createPod(), pod)
		t.Errorf("pod was mutated by CreatePodSecurityContext. diff:\n%s", diff)
	}
	if !reflect.DeepEqual(createSCC(), scc) {
		t.Error("scc was mutated by CreatePodSecurityContext")
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

	// Create an SCC with strategies that will populate a blank security context
	createSCC := func() *api.SecurityContextConstraints {
		var uid int64 = 1
		return &api.SecurityContextConstraints{
			ObjectMeta: api.ObjectMeta{
				Name: "scc-sa",
			},
			RunAsUser: api.RunAsUserStrategyOptions{
				Type: api.RunAsUserStrategyMustRunAs,
				UID:  &uid,
			},
			SELinuxContext: api.SELinuxContextStrategyOptions{
				Type:           api.SELinuxStrategyMustRunAs,
				SELinuxOptions: &api.SELinuxOptions{User: "you"},
			},
			// these are pod mutating strategies that are tested above
			FSGroup: api.FSGroupStrategyOptions{
				Type: api.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: api.SupplementalGroupsStrategyOptions{
				Type: api.SupplementalGroupsStrategyRunAsAny,
			},
		}
	}

	pod := createPod()
	scc := createSCC()

	provider, err := NewSimpleProvider(scc)
	if err != nil {
		t.Fatal("unable to create provider %v", err)
	}
	sc, err := provider.CreateContainerSecurityContext(pod, &pod.Spec.Containers[0])
	if err != nil {
		t.Fatal("unable to create provider %v", err)
	}

	// The generated security context should have filled in missing options, so they should differ
	if reflect.DeepEqual(sc, &pod.Spec.Containers[0].SecurityContext) {
		t.Error("expected created security context to be different than container's, but they were identical")
	}

	// Creating the provider or the security context should not have mutated the scc or pod
	if !reflect.DeepEqual(createPod(), pod) {
		diff := util.ObjectDiff(createPod(), pod)
		t.Errorf("pod was mutated by CreateContainerSecurityContext. diff:\n%s", diff)
	}
	if !reflect.DeepEqual(createSCC(), scc) {
		t.Error("scc was mutated by CreateContainerSecurityContext")
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
	failSupplementalGroupSCC := defaultSCC()
	failSupplementalGroupSCC.SupplementalGroups = api.SupplementalGroupsStrategyOptions {
		Type: api.SupplementalGroupsStrategyMustRunAs,
		Ranges: []api.IDRange{
			{Min: 1, Max: 1},
		},
	}

	failFSGroupPod := defaultPod()
	fsGroup := int64(999)
	failFSGroupPod.Spec.SecurityContext.FSGroup = &fsGroup
	failFSGroupSCC := defaultSCC()
	failFSGroupSCC.FSGroup = api.FSGroupStrategyOptions {
		Type: api.FSGroupStrategyMustRunAs,
		Ranges: []api.IDRange{
			{Min: 1, Max: 1},
		},
	}

	failNilSELinuxPod := defaultPod()
	failSELinuxSCC := defaultSCC()
	failSELinuxSCC.SELinuxContext.Type = api.SELinuxStrategyMustRunAs
	failSELinuxSCC.SELinuxContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "foo",
	}

	failInvalidSELinuxPod := defaultPod()
	failInvalidSELinuxPod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "bar",
	}

	errorCases := map[string]struct {
		pod           *api.Pod
		scc           *api.SecurityContextConstraints
		expectedError string
	}{
		"failHostNetworkSCC": {
			pod:           failHostNetworkPod,
			scc:           defaultSCC(),
			expectedError: "Host network is not allowed to be used",
		},
		"failHostPIDSCC": {
			pod:           failHostPIDPod,
			scc:           defaultSCC(),
			expectedError: "Host PID is not allowed to be used",
		},
		"failHostIPCSCC": {
			pod:           failHostIPCPod,
			scc:           defaultSCC(),
			expectedError: "Host IPC is not allowed to be used",
		},
		"failSupplementalGroupOutOfRange": {
			pod: failSupplementalGroupPod,
			scc: failSupplementalGroupSCC,
			expectedError: "999 is not an allowed group",
		},
		"failSupplementalGroupEmpty": {
			pod: defaultPod(),
			scc: failSupplementalGroupSCC,
			expectedError: "unable to validate empty groups against required ranges",
		},
		"failFSGroupOutOfRange": {
			pod: failFSGroupPod,
			scc: failFSGroupSCC,
			expectedError: "999 is not an allowed group",
		},
		"failFSGroupEmpty": {
			pod: defaultPod(),
			scc: failFSGroupSCC,
			expectedError: "unable to validate empty groups against required ranges",
		},
		"failNilSELinux": {
			pod: failNilSELinuxPod,
			scc: failSELinuxSCC,
			expectedError: "unable to validate nil seLinuxOptions",
		},
		"failInvalidSELinux": {
			pod: failInvalidSELinuxPod,
			scc: failSELinuxSCC,
			expectedError: "does not match required level.  Found bar, wanted foo",
		},
	}
	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.scc)
		if err != nil {
			t.Fatal("unable to create provider %v", err)
		}
		errs := provider.ValidatePodSecurityContext(v.pod)
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
	failUserSCC := defaultSCC()
	var uid int64 = 999
	var badUID int64 = 1
	failUserSCC.RunAsUser = api.RunAsUserStrategyOptions{
		Type: api.RunAsUserStrategyMustRunAs,
		UID:  &uid,
	}
	failUserPod := defaultPod()
	failUserPod.Spec.Containers[0].SecurityContext.RunAsUser = &badUID

	// fail selinux strat
	failSELinuxSCC := defaultSCC()
	failSELinuxSCC.SELinuxContext = api.SELinuxContextStrategyOptions{
		Type: api.SELinuxStrategyMustRunAs,
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

	errorCases := map[string]struct {
		pod           *api.Pod
		scc           *api.SecurityContextConstraints
		expectedError string
	}{
		"failUserSCC": {
			pod:           failUserPod,
			scc:           failUserSCC,
			expectedError: "does not match required UID",
		},
		"failSELinuxSCC": {
			pod:           failSELinuxPod,
			scc:           failSELinuxSCC,
			expectedError: "does not match required level",
		},
		"failPrivSCC": {
			pod:           failPrivPod,
			scc:           defaultSCC(),
			expectedError: "Privileged containers are not allowed",
		},
		"failCapsSCC": {
			pod:           failCapsPod,
			scc:           defaultSCC(),
			expectedError: "Capability is not allowed to be added",
		},
		"failHostDirSCC": {
			pod:           failHostDirPod,
			scc:           defaultSCC(),
			expectedError: "Host Volumes are not allowed to be used",
		},
		"failHostPortSCC": {
			pod:           failHostPortPod,
			scc:           defaultSCC(),
			expectedError: "Host ports are not allowed to be used",
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.scc)
		if err != nil {
			t.Fatal("unable to create provider %v", err)
		}
		errs := provider.ValidateContainerSecurityContext(v.pod, &v.pod.Spec.Containers[0])
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
	hostNetworkSCC := defaultSCC()
	hostNetworkSCC.AllowHostNetwork = true
	hostNetworkPod := defaultPod()
	hostNetworkPod.Spec.SecurityContext.HostNetwork = true

	hostPIDSCC := defaultSCC()
	hostPIDSCC.AllowHostPID = true
	hostPIDPod := defaultPod()
	hostPIDPod.Spec.SecurityContext.HostPID = true

	hostIPCSCC := defaultSCC()
	hostIPCSCC.AllowHostIPC = true
	hostIPCPod := defaultPod()
	hostIPCPod.Spec.SecurityContext.HostIPC = true

	supGroupSCC := defaultSCC()
	supGroupSCC.SupplementalGroups = api.SupplementalGroupsStrategyOptions{
		Type: api.SupplementalGroupsStrategyMustRunAs,
		Ranges: []api.IDRange{
			{Min: 1, Max: 5},
		},
	}
	supGroupPod := defaultPod()
	supGroupPod.Spec.SecurityContext.SupplementalGroups = []int64{3}

	fsGroupSCC := defaultSCC()
	fsGroupSCC.FSGroup = api.FSGroupStrategyOptions{
		Type: api.FSGroupStrategyMustRunAs,
		Ranges: []api.IDRange{
			{Min: 1, Max: 5},
		},
	}
	fsGroupPod := defaultPod()
	fsGroup := int64(3)
	fsGroupPod.Spec.SecurityContext.FSGroup = &fsGroup

	seLinuxPod := defaultPod()
	seLinuxPod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		User: "user",
		Role: "role",
		Type: "type",
		Level: "level",
	}
	seLinuxSCC := defaultSCC()
	seLinuxSCC.SELinuxContext.Type = api.SELinuxStrategyMustRunAs
	seLinuxSCC.SELinuxContext.SELinuxOptions = &api.SELinuxOptions{
		User: "user",
		Role: "role",
		Type: "type",
		Level: "level",
	}

	errorCases := map[string]struct {
		pod *api.Pod
		scc *api.SecurityContextConstraints
	}{
		"pass hostNetwork validating SCC": {
			pod: hostNetworkPod,
			scc: hostNetworkSCC,
		},
		"pass hostPID validating SCC": {
			pod: hostPIDPod,
			scc: hostPIDSCC,
		},
		"pass hostIPC validating SCC": {
			pod: hostIPCPod,
			scc: hostIPCSCC,
		},
		"pass supplemental group validating SCC": {
			pod: supGroupPod,
			scc: supGroupSCC,
		},
		"pass fs group validating SCC": {
			pod: fsGroupPod,
			scc: fsGroupSCC,
		},
		"pass selinux validating SCC": {
			pod: seLinuxPod,
			scc: seLinuxSCC,
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.scc)
		if err != nil {
			t.Fatal("unable to create provider %v", err)
		}
		errs := provider.ValidatePodSecurityContext(v.pod)
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
	userSCC := defaultSCC()
	var uid int64 = 999
	userSCC.RunAsUser = api.RunAsUserStrategyOptions{
		Type: api.RunAsUserStrategyMustRunAs,
		UID:  &uid,
	}
	userPod := defaultPod()
	userPod.Spec.Containers[0].SecurityContext.RunAsUser = &uid

	// fail selinux strat
	seLinuxSCC := defaultSCC()
	seLinuxSCC.SELinuxContext = api.SELinuxContextStrategyOptions{
		Type: api.SELinuxStrategyMustRunAs,
		SELinuxOptions: &api.SELinuxOptions{
			Level: "foo",
		},
	}
	seLinuxPod := defaultPod()
	seLinuxPod.Spec.Containers[0].SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "foo",
	}

	privSCC := defaultSCC()
	privSCC.AllowPrivilegedContainer = true
	privPod := defaultPod()
	var priv bool = true
	privPod.Spec.Containers[0].SecurityContext.Privileged = &priv

	capsSCC := defaultSCC()
	capsSCC.AllowedCapabilities = []api.Capability{"foo"}
	capsPod := defaultPod()
	capsPod.Spec.Containers[0].SecurityContext.Capabilities = &api.Capabilities{
		Add: []api.Capability{"foo"},
	}

	hostDirSCC := defaultSCC()
	hostDirSCC.AllowHostDirVolumePlugin = true
	hostDirPod := defaultPod()
	hostDirPod.Spec.Volumes = []api.Volume{
		{
			Name: "bad volume",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{},
			},
		},
	}

	hostPortSCC := defaultSCC()
	hostPortSCC.AllowHostPorts = true
	hostPortPod := defaultPod()
	hostPortPod.Spec.Containers[0].Ports = []api.ContainerPort{{HostPort: 1}}

	errorCases := map[string]struct {
		pod *api.Pod
		scc *api.SecurityContextConstraints
	}{
		"pass user must run as SCC": {
			pod: userPod,
			scc: userSCC,
		},
		"pass seLinux must run as SCC": {
			pod: seLinuxPod,
			scc: seLinuxSCC,
		},
		"pass priv validating SCC": {
			pod: privPod,
			scc: privSCC,
		},
		"pass caps validating SCC": {
			pod: capsPod,
			scc: capsSCC,
		},
		"pass hostDir validating SCC": {
			pod: hostDirPod,
			scc: hostDirSCC,
		},
		"pass hostPort validating SCC": {
			pod: hostPortPod,
			scc: hostPortSCC,
		},
	}

	for k, v := range errorCases {
		provider, err := NewSimpleProvider(v.scc)
		if err != nil {
			t.Fatal("unable to create provider %v", err)
		}
		errs := provider.ValidateContainerSecurityContext(v.pod, &v.pod.Spec.Containers[0])
		if len(errs) != 0 {
			t.Errorf("%s expected validation pass but received errors %v", k, errs)
			continue
		}
	}
}

func defaultSCC() *api.SecurityContextConstraints {
	return &api.SecurityContextConstraints{
		ObjectMeta: api.ObjectMeta{
			Name: "scc-sa",
		},
		RunAsUser: api.RunAsUserStrategyOptions{
			Type: api.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: api.SELinuxContextStrategyOptions{
			Type: api.SELinuxStrategyRunAsAny,
		},
		FSGroup: api.FSGroupStrategyOptions{
			Type: api.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: api.SupplementalGroupsStrategyOptions{
			Type: api.SupplementalGroupsStrategyRunAsAny,
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
