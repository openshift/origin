package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// SecurityContextConstraints governs the ability to make requests that affect the SecurityContext
// that will be applied to a container.
type SecurityContextConstraints struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// AllowPrivilegedContainer determines if a container can request to be run as privileged.
	AllowPrivilegedContainer bool
	// AllowedCapabilities is a list of capabilities that can be requested to add to the container.
	AllowedCapabilities []kapi.Capability
	// AllowHostDirVolumePlugin determines if the policy allow containers to use the HostDir volume plugin
	AllowHostDirVolumePlugin bool
	// AllowHostNetwork determines if the policy allows the use of HostNetwork in the pod spec.
	AllowHostNetwork bool
	// AllowHostPorts determines if the policy allows host ports in the containers.
	AllowHostPorts bool
	// SELinuxContext is the strategy that will dictate what labels will be set in the SecurityContext.
	SELinuxContext SELinuxContextStrategyOptions
	// RunAsUser is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	RunAsUser RunAsUserStrategyOptions

	// The users who have permissions to use this security context constraints
	Users []string
	// The groups that have permission to use this security context constraints
	Groups []string
}

// SELinuxContextStrategyOptions defines the strategy type and any options used to create the strategy.
type SELinuxContextStrategyOptions struct {
	// Type is the strategy that will dictate what SELinux context is used in the SecurityContext.
	Type SELinuxContextStrategyType
	// seLinuxOptions required to run as; required for MustRunAs
	SELinuxOptions *kapi.SELinuxOptions
}

// RunAsUserStrategyOptions defines the strategy type and any options used to create the strategy.
type RunAsUserStrategyOptions struct {
	// Type is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	Type RunAsUserStrategyType
	// UID is the user id that containers must run as.  Required for the MustRunAs strategy if not using
	// namespace/service account allocated uids.
	UID *int64
	// UIDRangeMin defines the min value for a strategy that allocates by range.
	UIDRangeMin *int64
	// UIDRangeMax defines the max value for a strategy that allocates by range.
	UIDRangeMax *int64
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
	kapi.TypeMeta
	kapi.ListMeta

	Items []SecurityContextConstraints
}
