package security

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

// +genclient=true
// +nonNamespaced=true

// SecurityContextConstraints governs the ability to make requests that affect the SecurityContext
// that will be applied to a container.
type SecurityContextConstraints struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Priority influences the sort order of SCCs when evaluating which SCCs to try first for
	// a given pod request based on access in the Users and Groups fields.  The higher the int, the
	// higher priority.  If scores for multiple SCCs are equal they will be sorted by name.
	Priority *int32

	// AllowPrivilegedContainer determines if a container can request to be run as privileged.
	AllowPrivilegedContainer bool
	// DefaultAddCapabilities is the default set of capabilities that will be added to the container
	// unless the pod spec specifically drops the capability.  You may not list a capabiility in both
	// DefaultAddCapabilities and RequiredDropCapabilities.
	DefaultAddCapabilities []kapi.Capability
	// RequiredDropCapabilities are the capabilities that will be dropped from the container.  These
	// are required to be dropped and cannot be added.
	RequiredDropCapabilities []kapi.Capability
	// AllowedCapabilities is a list of capabilities that can be requested to add to the container.
	// Capabilities in this field maybe added at the pod author's discretion.
	// You must not list a capability in both AllowedCapabilities and RequiredDropCapabilities.
	// To allow all capabilities you may use '*'.
	AllowedCapabilities []kapi.Capability
	// Volumes is a white list of allowed volume plugins.  FSType corresponds directly with the field names
	// of a VolumeSource (azureFile, configMap, emptyDir).  To allow all volumes you may use "*".
	// To allow no volumes, set to ["none"].
	Volumes []FSType
	// AllowHostNetwork determines if the policy allows the use of HostNetwork in the pod spec.
	AllowHostNetwork bool
	// AllowHostPorts determines if the policy allows host ports in the containers.
	AllowHostPorts bool
	// AllowHostPID determines if the policy allows host pid in the containers.
	AllowHostPID bool
	// AllowHostIPC determines if the policy allows host ipc in the containers.
	AllowHostIPC bool
	// SELinuxContext is the strategy that will dictate what labels will be set in the SecurityContext.
	SELinuxContext SELinuxContextStrategyOptions
	// RunAsUser is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	RunAsUser RunAsUserStrategyOptions
	// SupplementalGroups is the strategy that will dictate what supplemental groups are used by the SecurityContext.
	SupplementalGroups SupplementalGroupsStrategyOptions
	// FSGroup is the strategy that will dictate what fs group is used by the SecurityContext.
	FSGroup FSGroupStrategyOptions
	// ReadOnlyRootFilesystem when set to true will force containers to run with a read only root file
	// system.  If the container specifically requests to run with a non-read only root file system
	// the SCC should deny the pod.
	// If set to false the container may run with a read only root file system if it wishes but it
	// will not be forced to.
	ReadOnlyRootFilesystem bool
	// SeccompProfiles lists the allowed profiles that may be set for the pod or
	// container's seccomp annotations.  An unset (nil) or empty value means that no profiles may
	// be specifid by the pod or container.	The wildcard '*' may be used to allow all profiles.  When
	// used to generate a value for a pod the first non-wildcard profile will be used as
	// the default.
	SeccompProfiles []string

	// The users who have permissions to use this security context constraints
	Users []string
	// The groups that have permission to use this security context constraints
	Groups []string
}

// FS Type gives strong typing to different file systems that are used by volumes.
type FSType string

var (
	FSTypeAzureFile             FSType = "azureFile"
	FSTypeAzureDisk             FSType = "azureDisk"
	FSTypeFlocker               FSType = "flocker"
	FSTypeFlexVolume            FSType = "flexVolume"
	FSTypeHostPath              FSType = "hostPath"
	FSTypeEmptyDir              FSType = "emptyDir"
	FSTypeGCEPersistentDisk     FSType = "gcePersistentDisk"
	FSTypeAWSElasticBlockStore  FSType = "awsElasticBlockStore"
	FSTypeGitRepo               FSType = "gitRepo"
	FSTypeSecret                FSType = "secret"
	FSTypeNFS                   FSType = "nfs"
	FSTypeISCSI                 FSType = "iscsi"
	FSTypeGlusterfs             FSType = "glusterfs"
	FSTypePersistentVolumeClaim FSType = "persistentVolumeClaim"
	FSTypeRBD                   FSType = "rbd"
	FSTypeCinder                FSType = "cinder"
	FSTypeCephFS                FSType = "cephFS"
	FSTypeDownwardAPI           FSType = "downwardAPI"
	FSTypeFC                    FSType = "fc"
	FSTypeConfigMap             FSType = "configMap"
	FSTypeVsphereVolume         FSType = "vsphere"
	FSTypeQuobyte               FSType = "quobyte"
	FSTypePhotonPersistentDisk  FSType = "photonPersistentDisk"
	FSProjected                 FSType = "projected"
	FSPortworxVolume            FSType = "portworxVolume"
	FSScaleIO                   FSType = "scaleIO"
	FSStorageOS                 FSType = "storageOS"
	FSTypeAll                   FSType = "*"
	FSTypeNone                  FSType = "none"
)

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

// FSGroupStrategyOptions defines the strategy type and options used to create the strategy.
type FSGroupStrategyOptions struct {
	// Type is the strategy that will dictate what FSGroup is used in the SecurityContext.
	Type FSGroupStrategyType
	// Ranges are the allowed ranges of fs groups.  If you would like to force a single
	// fs group then supply a single range with the same start and end.
	Ranges []IDRange
}

// SupplementalGroupsStrategyOptions defines the strategy type and options used to create the strategy.
type SupplementalGroupsStrategyOptions struct {
	// Type is the strategy that will dictate what supplemental groups is used in the SecurityContext.
	Type SupplementalGroupsStrategyType
	// Ranges are the allowed ranges of supplemental groups.  If you would like to force a single
	// supplemental group then supply a single range with the same start and end.
	Ranges []IDRange
}

// IDRange provides a min/max of an allowed range of IDs.
// TODO: this could be reused for UIDs.
type IDRange struct {
	// Min is the start of the range, inclusive.
	Min int64
	// Max is the end of the range, inclusive.
	Max int64
}

// SELinuxContextStrategyType denotes strategy types for generating SELinux options for a
// SecurityContext
type SELinuxContextStrategyType string

// RunAsUserStrategyType denotes strategy types for generating RunAsUser values for a
// SecurityContext
type RunAsUserStrategyType string

// SupplementalGroupsStrategyType denotes strategy types for determining valid supplemental
// groups for a SecurityContext.
type SupplementalGroupsStrategyType string

// FSGroupStrategyType denotes strategy types for generating FSGroup values for a
// SecurityContext
type FSGroupStrategyType string

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

	// container must have FSGroup of X applied.
	FSGroupStrategyMustRunAs FSGroupStrategyType = "MustRunAs"
	// container may make requests for any FSGroup labels.
	FSGroupStrategyRunAsAny FSGroupStrategyType = "RunAsAny"

	// container must run as a particular gid.
	SupplementalGroupsStrategyMustRunAs SupplementalGroupsStrategyType = "MustRunAs"
	// container may make requests for any gid.
	SupplementalGroupsStrategyRunAsAny SupplementalGroupsStrategyType = "RunAsAny"
)

// SecurityContextConstraintsList is a list of SecurityContextConstraints objects
type SecurityContextConstraintsList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []SecurityContextConstraints
}

// PodSecurityPolicySubjectReview checks whether a particular user/SA tuple can create the PodTemplateSpec.
type PodSecurityPolicySubjectReview struct {
	metav1.TypeMeta

	// Spec defines specification for the PodSecurityPolicySubjectReview.
	Spec PodSecurityPolicySubjectReviewSpec

	// Status represents the current information/status for the PodSecurityPolicySubjectReview.
	Status PodSecurityPolicySubjectReviewStatus
}

// PodSecurityPolicySubjectReviewSpec defines specification for PodSecurityPolicySubjectReview
type PodSecurityPolicySubjectReviewSpec struct {
	// Template is the PodTemplateSpec to check. If PodTemplateSpec.Spec.ServiceAccountName is empty it will not be defaulted.
	// If its non-empty, it will be checked.
	Template kapi.PodTemplateSpec

	// User is the user you're testing for.
	// If you specify "User" but not "Group", then is it interpreted as "What if User were not a member of any groups.
	// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodTemplateSpec.
	User string

	// Groups is the groups you're testing for.
	Groups []string
}

// PodSecurityPolicySubjectReviewStatus contains information/status for PodSecurityPolicySubjectReview.
type PodSecurityPolicySubjectReviewStatus struct {
	// AllowedBy is a reference to the rule that allows the PodTemplateSpec.
	// A rule can be a SecurityContextConstraint or a PodSecurityPolicy
	// A `nil`, indicates that it was denied.
	AllowedBy *kapi.ObjectReference

	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available.
	Reason string

	// Template is the PodTemplateSpec after the defaulting is applied.
	Template kapi.PodTemplateSpec
}

// PodSecurityPolicySelfSubjectReview checks whether this user/SA tuple can create the PodTemplateSpec.
type PodSecurityPolicySelfSubjectReview struct {
	metav1.TypeMeta

	// Spec defines specification the PodSecurityPolicySelfSubjectReview.
	Spec PodSecurityPolicySelfSubjectReviewSpec

	// Status represents the current information/status for the PodSecurityPolicySelfSubjectReview.
	Status PodSecurityPolicySubjectReviewStatus
}

// PodSecurityPolicySelfSubjectReviewSpec contains specification for PodSecurityPolicySelfSubjectReview.
type PodSecurityPolicySelfSubjectReviewSpec struct {
	// Template is the PodTemplateSpec to check.
	Template kapi.PodTemplateSpec
}

// PodSecurityPolicyReview checks which service accounts (not users, since that would be cluster-wide) can create the `PodTemplateSpec` in question.
type PodSecurityPolicyReview struct {
	metav1.TypeMeta

	// Spec is the PodSecurityPolicy to check.
	Spec PodSecurityPolicyReviewSpec

	// Status represents the current information/status for the PodSecurityPolicyReview.
	Status PodSecurityPolicyReviewStatus
}

// PodSecurityPolicyReviewSpec defines specification for PodSecurityPolicyReview
type PodSecurityPolicyReviewSpec struct {
	// Template is the PodTemplateSpec to check. The PodTemplateSpec.Spec.ServiceAccountName field is used
	// if ServiceAccountNames is empty, unless the PodTemplateSpec.Spec.ServiceAccountName is empty,
	// in which case "default" is used.
	// If ServiceAccountNames is specified, PodTemplateSpec.Spec.ServiceAccountName is ignored.
	Template kapi.PodTemplateSpec

	// ServiceAccountNames is an optional set of ServiceAccounts to run the check with.
	// If ServiceAccountNames is empty, the PodTemplateSpec.Spec.ServiceAccountName is used,
	// unless it's empty, in which case "default" is used instead.
	// If ServiceAccountNames is specified, PodTemplateSpec.Spec.ServiceAccountName is ignored.
	ServiceAccountNames []string // TODO: find a way to express 'all service accounts'
}

// PodSecurityPolicyReviewStatus represents the status of PodSecurityPolicyReview.
type PodSecurityPolicyReviewStatus struct {
	// AllowedServiceAccounts returns the list of service accounts in *this* namespace that have the power to create the PodTemplateSpec.
	AllowedServiceAccounts []ServiceAccountPodSecurityPolicyReviewStatus
}

// ServiceAccountPodSecurityPolicyReviewStatus represents ServiceAccount name and related review status
type ServiceAccountPodSecurityPolicyReviewStatus struct {
	PodSecurityPolicySubjectReviewStatus

	// Name contains the allowed and the denied ServiceAccount name
	Name string
}
