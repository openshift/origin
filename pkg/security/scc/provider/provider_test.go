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

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/security/scc/api"
)

func TestCreateSecurityContextNonmutating(t *testing.T) {
	// Create a pod with a security context that needs filling in
	createPod := func() *kapi.Pod {
		return &kapi.Pod{
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{{
					SecurityContext: &kapi.SecurityContext{},
				}},
			},
		}
	}

	// Create an SCC with strategies that will populate a blank security context
	createSCC := func() *api.SecurityContextConstraints {
		var uid int64 = 1
		return &api.SecurityContextConstraints{
			ObjectMeta: kapi.ObjectMeta{
				Name: "scc-sa",
			},
			RunAsUser: api.RunAsUserStrategyOptions{
				Type: api.RunAsUserStrategyMustRunAs,
				UID:  &uid,
			},
			SELinuxContext: api.SELinuxContextStrategyOptions{
				Type:           api.SELinuxStrategyMustRunAs,
				SELinuxOptions: &kapi.SELinuxOptions{User: "you"},
			},
		}
	}

	pod := createPod()
	scc := createSCC()

	provider, err := NewSimpleProvider(scc)
	if err != nil {
		t.Fatal("unable to create provider %v", err)
	}
	sc, err := provider.CreateSecurityContext(pod, &pod.Spec.Containers[0])
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
		t.Errorf("pod was mutated by CreateSecurityContext. diff:\n%s", diff)
	}
	if !reflect.DeepEqual(createSCC(), scc) {
		t.Error("different")
	}
}

func TestValidateFailures(t *testing.T) {
	defaultSCC := func() *api.SecurityContextConstraints {
		return &api.SecurityContextConstraints{
			ObjectMeta: kapi.ObjectMeta{
				Name: "scc-sa",
			},
			RunAsUser: api.RunAsUserStrategyOptions{
				Type: api.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: api.SELinuxContextStrategyOptions{
				Type: api.SELinuxStrategyRunAsAny,
			},
		}
	}

	var notPriv bool = false
	defaultPod := func() *kapi.Pod {
		return &kapi.Pod{
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						SecurityContext: &kapi.SecurityContext{
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
		SELinuxOptions: &kapi.SELinuxOptions{
			Level: "foo",
		},
	}
	failSELinuxPod := defaultPod()
	failSELinuxPod.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "bar",
	}

	failPrivPod := defaultPod()
	var priv bool = true
	failPrivPod.Spec.Containers[0].SecurityContext.Privileged = &priv

	failCapsPod := defaultPod()
	failCapsPod.Spec.Containers[0].SecurityContext.Capabilities = &kapi.Capabilities{
		Add: []kapi.Capability{"foo"},
	}

	failHostDirPod := defaultPod()
	failHostDirPod.Spec.Volumes = []kapi.Volume{
		{
			Name: "bad volume",
			VolumeSource: kapi.VolumeSource{
				HostPath: &kapi.HostPathVolumeSource{},
			},
		},
	}

	failHostNetworkPod := defaultPod()
	failHostNetworkPod.Spec.HostNetwork = true

	failHostPortPod := defaultPod()
	failHostPortPod.Spec.Containers[0].Ports = []kapi.ContainerPort{{HostPort: 1}}

	errorCases := map[string]struct {
		pod           *kapi.Pod
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
		"failHostNetworkSCC": {
			pod:           failHostNetworkPod,
			scc:           defaultSCC(),
			expectedError: "Host network is not allowed to be used",
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
		errs := provider.ValidateSecurityContext(v.pod, &v.pod.Spec.Containers[0])
		if len(errs) == 0 {
			t.Errorf("%s expected validation failure but did not receive errors", k)
			continue
		}
		if !strings.Contains(errs[0].Error(), v.expectedError) {
			t.Errorf("%s received unexpected error %v", k, errs)
		}
	}
}

func TestValidateSuccess(t *testing.T) {
	defaultSCC := func() *api.SecurityContextConstraints {
		return &api.SecurityContextConstraints{
			ObjectMeta: kapi.ObjectMeta{
				Name: "scc-sa",
			},
			RunAsUser: api.RunAsUserStrategyOptions{
				Type: api.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: api.SELinuxContextStrategyOptions{
				Type: api.SELinuxStrategyRunAsAny,
			},
		}
	}

	var notPriv bool = false
	defaultPod := func() *kapi.Pod {
		return &kapi.Pod{
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						SecurityContext: &kapi.SecurityContext{
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
		SELinuxOptions: &kapi.SELinuxOptions{
			Level: "foo",
		},
	}
	seLinuxPod := defaultPod()
	seLinuxPod.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "foo",
	}

	privSCC := defaultSCC()
	privSCC.AllowPrivilegedContainer = true
	privPod := defaultPod()
	var priv bool = true
	privPod.Spec.Containers[0].SecurityContext.Privileged = &priv

	capsSCC := defaultSCC()
	capsSCC.AllowedCapabilities = []kapi.Capability{"foo"}
	capsPod := defaultPod()
	capsPod.Spec.Containers[0].SecurityContext.Capabilities = &kapi.Capabilities{
		Add: []kapi.Capability{"foo"},
	}

	hostDirSCC := defaultSCC()
	hostDirSCC.AllowHostDirVolumePlugin = true
	hostDirPod := defaultPod()
	hostDirPod.Spec.Volumes = []kapi.Volume{
		{
			Name: "bad volume",
			VolumeSource: kapi.VolumeSource{
				HostPath: &kapi.HostPathVolumeSource{},
			},
		},
	}

	hostNetworkSCC := defaultSCC()
	hostNetworkSCC.AllowHostNetwork = true
	hostNetworkPod := defaultPod()
	hostNetworkPod.Spec.HostNetwork = true

	hostPortSCC := defaultSCC()
	hostPortSCC.AllowHostPorts = true
	hostPortPod := defaultPod()
	hostPortPod.Spec.Containers[0].Ports = []kapi.ContainerPort{{HostPort: 1}}

	errorCases := map[string]struct {
		pod *kapi.Pod
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
		"pass hostNetwork validating SCC": {
			pod: hostNetworkPod,
			scc: hostNetworkSCC,
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
		errs := provider.ValidateSecurityContext(v.pod, &v.pod.Spec.Containers[0])
		if len(errs) != 0 {
			t.Errorf("%s expected validation pass but received errors %v", k, errs)
			continue
		}
	}
}
