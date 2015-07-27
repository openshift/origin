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

package securitycontext

import "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

// HasPrivilegedRequest returns the value of SecurityContext.Privileged, taking into account
// the possibility of nils
func HasPrivilegedRequest(container *api.Container) bool {
	if container.SecurityContext == nil {
		return false
	}
	if container.SecurityContext.Privileged == nil {
		return false
	}
	return *container.SecurityContext.Privileged
}

// HasCapabilitiesRequest returns true if Adds or Drops are defined in the security context
// capabilities, taking into account nils
func HasCapabilitiesRequest(container *api.Container) bool {
	if container.SecurityContext == nil {
		return false
	}
	if container.SecurityContext.Capabilities == nil {
		return false
	}
	return len(container.SecurityContext.Capabilities.Add) > 0 || len(container.SecurityContext.Capabilities.Drop) > 0
}

// HasNonRootUID returns true if the runAsUser is set and is greater than 0.
func HasRootUID(container *api.Container) bool {
	if container.SecurityContext == nil {
		return false
	}
	if container.SecurityContext.RunAsUser == nil {
		return false
	}
	return *container.SecurityContext.RunAsUser == 0
}

// HasRunAsUser determines if the sc's runAsUser field is set.
func HasRunAsUser(container *api.Container) bool {
	return container.SecurityContext != nil && container.SecurityContext.RunAsUser != nil
}

// HasRootRunAsUser returns true if the run as user is set and it is set to 0.
func HasRootRunAsUser(container *api.Container) bool {
	return HasRunAsUser(container) && HasRootUID(container)
}
