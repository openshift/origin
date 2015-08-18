package v1beta3

import (
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
)

// SecurityContextConstraints governs the ability to make requests that affect the SecurityContext
// that will be applied to a container.
type SecurityContextConstraints struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// AllowPrivilegedContainer determines if a container can request to be run as privileged.
	AllowPrivilegedContainer bool `json:"allowPrivilegedContainer" description:"allow containers to run as privileged"`
	// AllowedCapabilities is a list of capabilities that can be requested to add to the container.
	AllowedCapabilities []kapi.Capability `json:"allowedCapabilities" description:"capabilities that are allowed to be added"`
	// AllowHostDirVolumePlugin determines if the policy allow containers to use the HostDir volume plugin
	AllowHostDirVolumePlugin bool `json:"allowHostDirVolumePlugin" description:"allow the use of the host dir volume plugin"`
	// AllowHostNetwork determines if the policy allows the use of HostNetwork in the pod spec.
	AllowHostNetwork bool `json:"allowHostNetwork" description:"allow the use of the hostNetwork in the pod spec"`
	// AllowHostPorts determines if the policy allows host ports in the containers.
	AllowHostPorts bool `json:"allowHostPorts" description:"allow the use of the host ports in the containers"`
	// SELinuxContext is the strategy that will dictate what labels will be set in the SecurityContext.
	SELinuxContext SELinuxContextStrategyOptions `json:"seLinuxContext,omitempty" description:"strategy used to generate SELinuxOptions"`
	// RunAsUser is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	RunAsUser RunAsUserStrategyOptions `json:"runAsUser,omitempty" description:"strategy used to generate RunAsUser"`

	// The users who have permissions to use this security context constraints
	Users []string `json:"users,omitempty" description:"users allowed to use this SecurityContextConstraints"`
	// The groups that have permission to use this security context constraints
	Groups []string `json:"groups,omitempty" description:"groups allowed to use this SecurityContextConstraints"`
}

// SELinuxContextStrategyOptions defines the strategy type and any options used to create the strategy.
type SELinuxContextStrategyOptions struct {
	// Type is the strategy that will dictate what SELinux context is used in the SecurityContext.
	Type SELinuxContextStrategyType `json:"type,omitempty" description:"strategy used to generate the SELinux context"`
	// seLinuxOptions required to run as; required for MustRunAs
	SELinuxOptions *kapi.SELinuxOptions `json:"seLinuxOptions,omitempty" description:"seLinuxOptions required to run as; required for MustRunAs"`
}

// RunAsUserStrategyOptions defines the strategy type and any options used to create the strategy.
type RunAsUserStrategyOptions struct {
	// Type is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	Type RunAsUserStrategyType `json:"type,omitempty" description:"strategy used to generate RunAsUser"`
	// UID is the user id that containers must run as.  Required for the MustRunAs strategy if not using
	// namespace/service account allocated uids.
	UID *int64 `json:"uid,omitempty" description:"the uid to always run as; required for MustRunAs"`
	// UIDRangeMin defines the min value for a strategy that allocates by range.
	UIDRangeMin *int64 `json:"uidRangeMin,omitempty" description:"min value for range based allocators"`
	// UIDRangeMax defines the max value for a strategy that allocates by range.
	UIDRangeMax *int64 `json:"uidRangeMax,omitempty" description:"max value for range based allocators"`
}

// SELinuxContextStrategyType denotes strategy types for generating SELinux options for a
// SecurityContext
type SELinuxContextStrategyType string

// RunAsUserStrategyType denotes strategy types for generating RunAsUser values for a
// SecurityContext
type RunAsUserStrategyType string

const (
	// container must have SELinux labels of X applied.
	SELinuxStrategyMustRunAs SELinuxContextStrategyType = "MustRunAs"
	// container may make requests for any SELinux context labels.
	SELinuxStrategyRunAsAny SELinuxContextStrategyType = "RunAsAny"

	// container must run as a particular uid.
	RunAsUserStrategyMustRunAs RunAsUserStrategyType = "MustRunAs"
	// container must run as a particular uid.
	RunAsUserStrategyMustRunAsRange RunAsUserStrategyType = "MustRunAsRange"
	// container must run as a non-root uid
	RunAsUserStrategyMustRunAsNonRoot RunAsUserStrategyType = "MustRunAsNonRoot"
	// container may make requests for any uid.
	RunAsUserStrategyRunAsAny RunAsUserStrategyType = "RunAsAny"
)

// SecurityContextConstraintsList is a list of SecurityContextConstraints objects
type SecurityContextConstraintsList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []SecurityContextConstraints `json:"items" description:"list of security context constraints objects"`
}
