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
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/securitycontextconstraints/selinux"
	"k8s.io/kubernetes/pkg/securitycontextconstraints/user"
	"k8s.io/kubernetes/pkg/util/fielderrors"
)

// simpleProvider is the default implementation of SecurityContextConstraintsProvider
type simpleProvider struct {
	scc               *api.SecurityContextConstraints
	runAsUserStrategy user.RunAsUserSecurityContextConstraintsStrategy
	seLinuxStrategy   selinux.SELinuxSecurityContextConstraintsStrategy
}

// ensure we implement the interface correctly.
var _ SecurityContextConstraintsProvider = &simpleProvider{}

// NewSimpleProvider creates a new SecurityContextConstraintsProvider instance.
func NewSimpleProvider(scc *api.SecurityContextConstraints) (SecurityContextConstraintsProvider, error) {
	if scc == nil {
		return nil, fmt.Errorf("NewSimpleProvider requires a SecurityContextConstraints")
	}

	var userStrat user.RunAsUserSecurityContextConstraintsStrategy = nil
	var err error = nil
	switch scc.RunAsUser.Type {
	case api.RunAsUserStrategyMustRunAs:
		userStrat, err = user.NewMustRunAs(&scc.RunAsUser)
	case api.RunAsUserStrategyMustRunAsRange:
		userStrat, err = user.NewMustRunAsRange(&scc.RunAsUser)
	case api.RunAsUserStrategyMustRunAsNonRoot:
		userStrat, err = user.NewRunAsNonRoot(&scc.RunAsUser)
	case api.RunAsUserStrategyRunAsAny:
		userStrat, err = user.NewRunAsAny(&scc.RunAsUser)
	default:
		err = fmt.Errorf("Unrecognized RunAsUser strategy type %s", scc.RunAsUser.Type)
	}
	if err != nil {
		return nil, err
	}

	var seLinuxStrat selinux.SELinuxSecurityContextConstraintsStrategy = nil
	err = nil
	switch scc.SELinuxContext.Type {
	case api.SELinuxStrategyMustRunAs:
		seLinuxStrat, err = selinux.NewMustRunAs(&scc.SELinuxContext)
	case api.SELinuxStrategyRunAsAny:
		seLinuxStrat, err = selinux.NewRunAsAny(&scc.SELinuxContext)
	default:
		err = fmt.Errorf("Unrecognized SELinuxContext strategy type %s", scc.SELinuxContext.Type)
	}
	if err != nil {
		return nil, err
	}

	return &simpleProvider{
		scc:               scc,
		runAsUserStrategy: userStrat,
		seLinuxStrategy:   seLinuxStrat,
	}, nil
}

// Create a SecurityContext based on the given constraints.  If a setting is already set on the
// container's security context then it will not be changed.  Validation should be used after
// the context is created to ensure it complies with the required restrictions.
//
// NOTE: this method works on a copy of the SC of the container.  It is up to the caller to apply
// the SC if validation passes.
func (s *simpleProvider) CreateSecurityContext(pod *api.Pod, container *api.Container) (*api.SecurityContext, error) {
	var sc *api.SecurityContext = nil
	if container.SecurityContext != nil {
		// work with a copy of the original
		copy := *container.SecurityContext
		sc = &copy
	} else {
		sc = &api.SecurityContext{}
	}
	if sc.RunAsUser == nil {
		uid, err := s.runAsUserStrategy.Generate(pod, container)
		if err != nil {
			return nil, err
		}
		sc.RunAsUser = uid
	}

	if sc.SELinuxOptions == nil {
		seLinux, err := s.seLinuxStrategy.Generate(pod, container)
		if err != nil {
			return nil, err
		}
		sc.SELinuxOptions = seLinux
	}

	if sc.Privileged == nil {
		priv := false
		sc.Privileged = &priv
	}

	// if we're using the non-root strategy set the marker that this container should not be
	// run as root which will signal to the kubelet to do a final check either on the runAsUser
	// or, if runAsUser is not set, the image
	if s.scc.RunAsUser.Type == api.RunAsUserStrategyMustRunAsNonRoot {
		sc.RunAsNonRoot = true
	}

	// No need to touch capabilities, they will validate or not.
	return sc, nil
}

// Ensure a container's SecurityContext is in compliance with the given constraints
func (s *simpleProvider) ValidateSecurityContext(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if container.SecurityContext == nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("securityContext", container.SecurityContext, "No security context is set"))
		return allErrs
	}

	sc := container.SecurityContext
	allErrs = append(allErrs, s.runAsUserStrategy.Validate(pod, container)...)
	allErrs = append(allErrs, s.seLinuxStrategy.Validate(pod, container)...)

	if !s.scc.AllowPrivilegedContainer && *sc.Privileged {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("privileged", *sc.Privileged, "Privileged containers are not allowed"))
	}

	if sc.Capabilities != nil && len(sc.Capabilities.Add) > 0 {
		for _, cap := range sc.Capabilities.Add {
			found := false
			for _, allowedCap := range s.scc.AllowedCapabilities {
				if cap == allowedCap {
					found = true
					break
				}
			}
			if !found {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("capabilities.add", cap, "Capability is not allowed to be added"))
			}
		}
	}

	if !s.scc.AllowHostDirVolumePlugin {
		for _, v := range pod.Spec.Volumes {
			if v.HostPath != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("VolumeMounts", v.Name, "Host Volumes are not allowed to be used"))
			}
		}
	}

	if !s.scc.AllowHostNetwork && pod.Spec.SecurityContext.HostNetwork {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostNetwork", pod.Spec.SecurityContext.HostNetwork, "Host network is not allowed to be used"))
	}

	if !s.scc.AllowHostPorts {
		for idx, c := range pod.Spec.Containers {
			allErrs = append(allErrs, s.hasHostPort(&c).Prefix(fmt.Sprintf("containers.%d", idx))...)
		}
	}

	if !s.scc.AllowHostPID && pod.Spec.SecurityContext.HostPID {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostPID", pod.Spec.SecurityContext.HostPID, "Host PID is not allowed to be used"))
	}

	if !s.scc.AllowHostIPC && pod.Spec.SecurityContext.HostIPC {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostIPC", pod.Spec.SecurityContext.HostIPC, "Host IPC is not allowed to be used"))
	}

	return allErrs
}

// hasHostPort checks the port definitions on the container for HostPort > 0.
func (s *simpleProvider) hasHostPort(container *api.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	for _, cp := range container.Ports {
		if cp.HostPort > 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostPort", cp.HostPort, "Host ports are not allowed to be used"))
		}
	}
	return allErrs
}

// Get the name of the SCC that this provider was initialized with.
func (s *simpleProvider) GetSCCName() string {
	return s.scc.Name
}
