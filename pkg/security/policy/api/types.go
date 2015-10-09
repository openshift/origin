package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// SecurityContextConstraints is a deprecated type required to maintain backwards compatibility.
// WARNING: SecurityContextConstraints share some types with PodSecurityPolicy.  Do not make
// changes to PodSecurityPolicy without ensuring backwards compatibility.
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

	// The users who have permissions to use this pod security policy.
	Users []string
	// The groups that have permission to use this pod security policy.
	Groups []string
}

// SecurityContextConstraintsList is a list of SecurityContextConstraints objects
type SecurityContextConstraintsList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []SecurityContextConstraints
}

// PodSecurityPolicy governs the ability to make requests that affect the SecurityContext
// that will be applied to a container.
type PodSecurityPolicy struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Spec defines the policy enforced.
	Spec PodSecurityPolicySpec
}

// PodSecurityPolicySpec defines the policy enforced.
type PodSecurityPolicySpec struct {
	// Privileged determines if a pod can request to be run as privileged.
	Privileged bool
	// Capabilities is a list of capabilities that can be added.
	Capabilities []kapi.Capability
	// Volumes allows and disallows the use of different types of volume plugins.
	Volumes VolumeSecurityPolicy
	// HostNetwork determines if the policy allows the use of HostNetwork in the pod spec.
	HostNetwork bool
	// HostPorts determines which host port ranges are allowed to be exposed.
	HostPorts []HostPortRange
	// SELinuxContext is the strategy that will dictate the allowable labels that may be set.
	SELinuxContext SELinuxContextStrategyOptions
	// RunAsUser is the strategy that will dictate the allowable RunAsUser values that may be set.
	RunAsUser RunAsUserStrategyOptions

	// The users who have permissions to use this policy
	Users []string
	// The groups that have permission to use this policy
	Groups []string
}

// HostPortRange defines a range of host ports that will be enabled by a policy
// for pods to use.  It requires both the start and end to be defined.
type HostPortRange struct {
	// Start is the beginning of the port range which will be allowed.
	Start int
	// End is the end of the port range which will be allowed.
	End int
}

// VolumeSecurityPolicy allows and disallows the use of different types of volume plugins.
type VolumeSecurityPolicy struct {
	// HostPath allows or disallows the use of the HostPath volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#hostpath
	HostPath bool
	// EmptyDir allows or disallows the use of the EmptyDir volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#emptydir
	EmptyDir bool
	// GCEPersistentDisk allows or disallows the use of the GCEPersistentDisk volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#gcepersistentdisk
	GCEPersistentDisk bool
	// AWSElasticBlockStore allows or disallows the use of the AWSElasticBlockStore volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#awselasticblockstore
	AWSElasticBlockStore bool
	// GitRepo allows or disallows the use of the GitRepo volume plugin.
	GitRepo bool
	// Secret allows or disallows the use of the Secret volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#secrets
	Secret bool
	// NFS allows or disallows the use of the NFS volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#nfs
	NFS bool
	// ISCSI allows or disallows the use of the ISCSI volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/iscsi/README.md
	ISCSI bool
	// Glusterfs allows or disallows the use of the Glusterfs volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/glusterfs/README.md
	Glusterfs bool
	// PersistentVolumeClaim allows or disallows the use of the PersistentVolumeClaim volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/persistent-volumes.md#persistentvolumeclaims
	PersistentVolumeClaim bool
	// RBD allows or disallows the use of the RBD volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/rbd/README.md
	RBD bool
	// Cinder allows or disallows the use of the Cinder volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/mysql-cinder-pd/README.md
	Cinder bool
	// CephFS allows or disallows the use of the CephFS volume plugin.
	CephFS bool
	// DownwardAPI allows or disallows the use of the DownwardAPI volume plugin.
	DownwardAPI bool
	// FC allows or disallows the use of the FC volume plugin.
	FC bool
}

// SELinuxContextStrategyOptions defines the strategy type and any options used to create the strategy.
type SELinuxContextStrategyOptions struct {
	// Type is the strategy that will dictate what SELinux context is used in the SecurityContext.
	Type SELinuxContextStrategy
	// seLinuxOptions required to run as; required for MustRunAs
	SELinuxOptions *kapi.SELinuxOptions
}

// SELinuxContextStrategyType denotes strategy types for generating SELinux options for a
// SecurityContext
type SELinuxContextStrategy string

const (
	// container must have SELinux labels of X applied.
	SELinuxStrategyMustRunAs SELinuxContextStrategy = "MustRunAs"
	// container may make requests for any SELinux context labels.
	SELinuxStrategyRunAsAny SELinuxContextStrategy = "RunAsAny"
)

// RunAsUserStrategyOptions defines the strategy type and any options used to create the strategy.
type RunAsUserStrategyOptions struct {
	// Type is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	Type RunAsUserStrategy
	// UID is the user id that containers must run as.  Required for the MustRunAs strategy if not using
	// namespace/service account allocated uids.
	UID *int64
	// UIDRangeMin defines the min value for a strategy that allocates by range.
	UIDRangeMin *int64
	// UIDRangeMax defines the max value for a strategy that allocates by range.
	UIDRangeMax *int64
}

// RunAsUserStrategyType denotes strategy types for generating RunAsUser values for a
// SecurityContext
type RunAsUserStrategy string

const (
	// container must run as a particular uid.
	RunAsUserStrategyMustRunAs RunAsUserStrategy = "MustRunAs"
	// container must run as a particular uid.
	RunAsUserStrategyMustRunAsRange RunAsUserStrategy = "MustRunAsRange"
	// container must run as a non-root uid
	RunAsUserStrategyMustRunAsNonRoot RunAsUserStrategy = "MustRunAsNonRoot"
	// container may make requests for any uid.
	RunAsUserStrategyRunAsAny RunAsUserStrategy = "RunAsAny"
)

// PodSecurityPolicyList is a list of PodSecurityPolicy objects
type PodSecurityPolicyList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []PodSecurityPolicy
}
