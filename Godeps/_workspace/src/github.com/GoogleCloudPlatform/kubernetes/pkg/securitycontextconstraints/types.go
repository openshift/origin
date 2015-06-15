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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
)

// SecurityContextConstraintsProvider provides the implementation to generate a new security
// context based on constraints or validate an existing security context against constraints.
type SecurityContextConstraintsProvider interface {
	// Create a SecurityContext based on the given constraints
	CreateSecurityContext(pod *api.Pod, container *api.Container) (*api.SecurityContext, error)
	// Ensure a container's SecurityContext is in compliance with the given constraints
	ValidateSecurityContext(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList
	// Get the name of the SCC that this provider was initialized with.
	GetSCCName() string
}
