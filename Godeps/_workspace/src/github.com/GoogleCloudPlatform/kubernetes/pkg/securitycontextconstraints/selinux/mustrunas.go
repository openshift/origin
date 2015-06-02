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

package selinux

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
)

type mustRunAs struct {
	opts *api.SELinuxContextStrategyOptions
}

func NewMustRunAs(options *api.SELinuxContextStrategyOptions) (SELinuxSecurityContextConstraintsStrategy, error) {
	if options == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxContextStrategyOptions")
	}
	if options.SELinuxOptions == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxOptions")
	}
	return &mustRunAs{
		opts: options,
	}, nil
}

// Generate creates the SELinuxOptions based on constraint rules.
func (s *mustRunAs) Generate(pod *api.Pod, container *api.Container) (*api.SELinuxOptions, error) {
	return s.opts.SELinuxOptions, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *mustRunAs) Validate(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if container.SecurityContext == nil {
		allErrs = append(allErrs, fmt.Errorf("Unable to validate nil security context for container %s", container.Name))
		return allErrs
	}
	if container.SecurityContext.SELinuxOptions == nil {
		allErrs = append(allErrs, fmt.Errorf("Unable to validate nil SELinuxOptions for container %s", container.Name))
		return allErrs
	}
	seLinux := container.SecurityContext.SELinuxOptions
	if seLinux.Level != s.opts.SELinuxOptions.Level {
		allErrs = append(allErrs, fmt.Errorf("SELinuxOptions.Level on container %s does not match required level.  Found %s, wanted %s", container.Name, seLinux.Level, s.opts.SELinuxOptions.Level))
	}
	if seLinux.Role != s.opts.SELinuxOptions.Role {
		allErrs = append(allErrs, fmt.Errorf("SELinuxOptions.Role on container %s does not match required role.  Found %s, wanted %s", container.Name, seLinux.Role, s.opts.SELinuxOptions.Role))
	}
	if seLinux.Type != s.opts.SELinuxOptions.Type {
		allErrs = append(allErrs, fmt.Errorf("SELinuxOptions.Type on container %s does not match required type.  Found %s, wanted %s", container.Name, seLinux.Type, s.opts.SELinuxOptions.Type))
	}
	if seLinux.User != s.opts.SELinuxOptions.User {
		allErrs = append(allErrs, fmt.Errorf("SELinuxOptions.User on container %s does not match required user.  Found %s, wanted %s", container.Name, seLinux.User, s.opts.SELinuxOptions.User))
	}

	return allErrs
}
