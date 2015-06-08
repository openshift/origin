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

package user

import (
	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
)

// mustRunAs implements the RunAsUserSecurityContextConstraintsStrategy interface
type mustRunAsRange struct {
	opts *api.RunAsUserStrategyOptions
}

// NewMustRunAs provides a strategy that requires the container to run as a specific UID in a range.
func NewMustRunAsRange(options *api.RunAsUserStrategyOptions) (RunAsUserSecurityContextConstraintsStrategy, error) {
	if options == nil {
		return nil, fmt.Errorf("MustRunAsRange requires run as user options")
	}
	if options.UIDRangeMin == nil {
		return nil, fmt.Errorf("MustRunAsRange requires a UIDRangeMin")
	}
	if options.UIDRangeMax == nil {
		return nil, fmt.Errorf("MustRunAsRange requires a UIDRangeMax")
	}
	return &mustRunAsRange{
		opts: options,
	}, nil
}

// Generate creates the uid based on policy rules.  MustRunAs returns the UIDRangeMin it is initialized with.
func (s *mustRunAsRange) Generate(pod *api.Pod, container *api.Container) (*int64, error) {
	return s.opts.UIDRangeMin, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *mustRunAsRange) Validate(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if container.SecurityContext == nil {
		allErrs = append(allErrs, fmt.Errorf("Unable to validate nil security context for container %s", container.Name))
		return allErrs
	}
	if container.SecurityContext.RunAsUser == nil {
		allErrs = append(allErrs, fmt.Errorf("Unable to validate nil RunAsUser for container %s", container.Name))
		return allErrs
	}

	if *container.SecurityContext.RunAsUser < *s.opts.UIDRangeMin || *container.SecurityContext.RunAsUser > *s.opts.UIDRangeMax {
		allErrs = append(allErrs, fmt.Errorf("UID on container %s does not match required range.  Found %d, required min: %d max: %d",
			container.Name,
			*container.SecurityContext.RunAsUser,
			*s.opts.UIDRangeMin,
			*s.opts.UIDRangeMax))
	}

	return allErrs
}
