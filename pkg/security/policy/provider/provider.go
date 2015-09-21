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

package provider

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/security/policy/api"
	"github.com/openshift/origin/pkg/security/policy/strategies/selinux"
	"github.com/openshift/origin/pkg/security/policy/strategies/user"
)

// simpleProvider is the default implementation of SecurityContextConstraintsProvider
type simpleProvider struct {
	psp               *api.PodSecurityPolicy
	runAsUserStrategy user.RunAsUserStrategy
	seLinuxStrategy   selinux.SELinuxStrategy
}

// ensure we implement the interface correctly.
var _ PodSecurityPolicyProvider = &simpleProvider{}

// NewSimpleProvider creates a new PodSecurityPolicyProvider instance.
func NewSimpleProvider(psp *api.PodSecurityPolicy) (PodSecurityPolicyProvider, error) {
	if psp == nil {
		return nil, fmt.Errorf("NewSimpleProvider requires a PodSecurityPolicy")
	}

	var userStrat user.RunAsUserStrategy = nil
	var err error = nil
	switch psp.Spec.RunAsUser.Type {
	case api.RunAsUserStrategyMustRunAs:
		userStrat, err = user.NewMustRunAs(&psp.Spec.RunAsUser)
	case api.RunAsUserStrategyMustRunAsRange:
		userStrat, err = user.NewMustRunAsRange(&psp.Spec.RunAsUser)
	case api.RunAsUserStrategyMustRunAsNonRoot:
		userStrat, err = user.NewRunAsNonRoot(&psp.Spec.RunAsUser)
	case api.RunAsUserStrategyRunAsAny:
		userStrat, err = user.NewRunAsAny(&psp.Spec.RunAsUser)
	default:
		err = fmt.Errorf("Unrecognized RunAsUser strategy type %s", psp.Spec.RunAsUser.Type)
	}
	if err != nil {
		return nil, err
	}

	var seLinuxStrat selinux.SELinuxStrategy = nil
	err = nil
	switch psp.Spec.SELinuxContext.Type {
	case api.SELinuxStrategyMustRunAs:
		seLinuxStrat, err = selinux.NewMustRunAs(&psp.Spec.SELinuxContext)
	case api.SELinuxStrategyRunAsAny:
		seLinuxStrat, err = selinux.NewRunAsAny(&psp.Spec.SELinuxContext)
	default:
		err = fmt.Errorf("Unrecognized SELinuxContext strategy type %s", psp.Spec.SELinuxContext.Type)
	}
	if err != nil {
		return nil, err
	}

	return &simpleProvider{
		psp:               psp,
		runAsUserStrategy: userStrat,
		seLinuxStrategy:   seLinuxStrat,
	}, nil
}

// Create a SecurityContext based on the given policy.  If a setting is already set on the
// container's security context then it will not be changed.  Validation should be used after
// the context is created to ensure it complies with the required restrictions.
//
// NOTE: this method works on a copy of the SC of the container.  It is up to the caller to apply
// the SC if validation passes.
func (s *simpleProvider) CreateSecurityContext(pod *kapi.Pod, container *kapi.Container) (*kapi.SecurityContext, error) {
	var sc *kapi.SecurityContext = nil
	if container.SecurityContext != nil {
		// work with a copy of the original
		copy := *container.SecurityContext
		sc = &copy
	} else {
		sc = &kapi.SecurityContext{}
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
	if s.psp.Spec.RunAsUser.Type == api.RunAsUserStrategyMustRunAsNonRoot {
		sc.RunAsNonRoot = true
	}

	// No need to touch capabilities, they will validate or not.
	return sc, nil
}

// Ensure a container's SecurityContext is in compliance with the given constraints
func (s *simpleProvider) ValidateSecurityContext(pod *kapi.Pod, container *kapi.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if container.SecurityContext == nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("securityContext", container.SecurityContext, "No security context is set"))
		return allErrs
	}

	sc := container.SecurityContext
	allErrs = append(allErrs, s.runAsUserStrategy.Validate(pod, container)...)
	allErrs = append(allErrs, s.seLinuxStrategy.Validate(pod, container)...)

	if !s.psp.Spec.Privileged && *sc.Privileged {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("privileged", *sc.Privileged, "Privileged containers are not allowed"))
	}

	if sc.Capabilities != nil && len(sc.Capabilities.Add) > 0 {
		for _, cap := range sc.Capabilities.Add {
			found := false
			for _, allowedCap := range s.psp.Spec.Capabilities {
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

	//TODO full support for volume plugins!
	if !s.psp.Spec.Volumes.HostPath {
		for _, v := range pod.Spec.Volumes {
			if v.HostPath != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("VolumeMounts", v.Name, "Host Volumes are not allowed to be used"))
			}
		}
	}

	if !s.psp.Spec.HostNetwork && pod.Spec.HostNetwork {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostNetwork", pod.Spec.HostNetwork, "Host network is not allowed to be used"))
	}

	for idx, c := range pod.Spec.Containers {
		allErrs = append(allErrs, s.validateHostPorts(&c).Prefix(fmt.Sprintf("containers.%d", idx))...)
	}

	return allErrs
}

// validateHostPorts checks that any requested host ports are allowed by this policy.
func (s *simpleProvider) validateHostPorts(container *kapi.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	for _, cp := range container.Ports {
		if !s.isAllowedHostPort(cp.HostPort) {
			msg := fmt.Sprintf("Host port %d is not allowed to be used", cp.HostPort)
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("hostPort", cp.HostPort, msg))
		}
	}
	return allErrs
}

// isAllowedHostPort returns true if the port falls within an allowable range.
func (s *simpleProvider) isAllowedHostPort(port int) bool {
	for _, p := range s.psp.Spec.HostPorts {
		if port >= p.Start && port <= p.End {
			return true
		}
	}
	return false
}

// Get the name of the psp that this provider was initialized with.
func (s *simpleProvider) GetPolicyName() string {
	return s.psp.Name
}
