package v1

import (
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// SecurityContextConstraints is a deprecated type required to maintain backwards compatibility.
// WARNING: SecurityContextConstraints share some types with PodSecurityPolicy.  Do not make
// changes to PodSecurityPolicy without ensuring backwards compatibility.
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

	// The users who have permissions to use this PodSecurityPolicy.
	Users []string `json:"users,omitempty" description:"users allowed to use this PodSecurityPolicy"`
	// The groups that have permission to use this PodSecurityPolicy.
	Groups []string `json:"groups,omitempty" description:"groups allowed to use this PodSecurityPolicy"`
}

// SecurityContextConstraintsList is a list of SecurityContextConstraints objects
type SecurityContextConstraintsList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []SecurityContextConstraints `json:"items" description:"list of SecurityContextConstraints objects"`
}

// PodSecurityPolicy governs the ability to make requests that affect the SecurityContext
// that will be applied to a container.
type PodSecurityPolicy struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the policy enforced.
	Spec PodSecurityPolicySpec `json:"spec,omitempty" description:"EXPERIMENTAL - pod security policy spec"`
}

// PodSecurityPolicySpec defines the policy enforced.
type PodSecurityPolicySpec struct {
	// Privileged determines if a container can request to be run as privileged.
	Privileged bool `json:"privileged,omitempty" description:"EXPERIMENTAL - allow containers to run as privileged"`
	// Capabilities is a list of capabilities that can be requested to add to the container.
	Capabilities []kapi.Capability `json:"capabilities,omitempty" description:"EXPERIMENTAL - capabilities that are allowed to be added"`
	// Volumes allows and disallows the use of different types of volume plugins.
	Volumes VolumeSecurityPolicy `json:"volumes,omitempty" description:"EXPERIMENTAL - volume plugins that are allowed to be used"`
	// HostNetwork determines if the policy allows the use of HostNetwork in the pod spec.
	HostNetwork bool `json:"hostNetwork,omitempty" description:"EXPERIMENTAL - allow the use of the hostNetwork in the pod spec"`
	// HostPorts determines if the policy allows host ports in the containers.
	HostPorts []HostPortRange `json:"hostPorts" description:"EXPERIMENTAL - allow the use of the host ports in the containers"`
	// SELinuxContext is the strategy that will dictate what labels will be set in the SecurityContext.
	SELinuxContext SELinuxContextStrategyOptions `json:"seLinuxContext,omitempty" description:"EXPERIMENTAL - strategy used to generate SELinuxOptions"`
	// RunAsUser is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	RunAsUser RunAsUserStrategyOptions `json:"runAsUser,omitempty" description:"EXPERIMENTAL - strategy used to generate RunAsUser"`

	// The users who have permissions to use this PodSecurityPolicy.
	Users []string `json:"users,omitempty" description:"EXPERIMENTAL - users allowed to use this PodSecurityPolicy"`
	// The groups that have permission to use this PodSecurityPolicy.
	Groups []string `json:"groups,omitempty" description:"EXPERIMENTAL - groups allowed to use this PodSecurityPolicy"`
}

// HostPortRange defines a range of host ports that will be enabled by a policy
// for pods to use.  It requires both the start and end to be defined.
type HostPortRange struct {
	// Start is the beginning of the port range which will be allowed.
	Start int `json:"start" description:"starting port of the range"`
	// End is the end of the port range which will be allowed.
	End int `json:"end" description:"ending port of the range"`
}

// VolumeSecurityPolicy allows and disallows the use of different types of volume plugins.
type VolumeSecurityPolicy struct {
	// HostPath allows or disallows the use of the HostPath volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#hostpath
	HostPath bool `json:"hostPath,omitempty" description:"allows or disallows the use of the HostPath volume plugin"`
	// EmptyDir allows or disallows the use of the EmptyDir volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#emptydir
	EmptyDir bool `json:"emptyDir,omitempty" description:"allows or disallows the use of the EmptyDir volume plugin"`
	// GCEPersistentDisk allows or disallows the use of the GCEPersistentDisk volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#gcepersistentdisk
	GCEPersistentDisk bool `json:"gcePersistentDisk,omitempty" description:"allows or disallows the use of the GCEPersistentDisk volume plugin"`
	// AWSElasticBlockStore allows or disallows the use of the AWSElasticBlockStore volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#awselasticblockstore
	AWSElasticBlockStore bool `json:"awsElasticBlockStore,omitempty" description:"allows or disallows the use of the AWSElasticBlockStore volume plugin"`
	// GitRepo allows or disallows the use of the GitRepo volume plugin.
	GitRepo bool `json:"gitRepo,omitempty" description:"allows or disallows the use of the GitRepo volume plugin"`
	// Secret allows or disallows the use of the Secret volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#secrets
	Secret bool `json:"secret,omitempty" description:"allows or disallows the use of the Secret volume plugin"`
	// NFS allows or disallows the use of the NFS volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/volumes.md#nfs
	NFS bool `json:"nfs,omitempty" description:"allows or disallows the use of the NFS volume plugin"`
	// ISCSI allows or disallows the use of the ISCSI volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/iscsi/README.md
	ISCSI bool `json:"iscsi,omitempty" description:"allows or disallows the use of the ISCSI volume plugin"`
	// Glusterfs allows or disallows the use of the Glusterfs volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/glusterfs/README.md
	Glusterfs bool `json:"glusterfs,omitempty" description:"allows or disallows the use of the Glusterfs volume plugin"`
	// PersistentVolumeClaim allows or disallows the use of the PersistentVolumeClaim volume plugin.
	// More info: http://releases.k8s.io/HEAD/docs/user-guide/persistent-volumes.md#persistentvolumeclaims
	PersistentVolumeClaim bool `json:"persistentVolumeClaim,omitempty" description:"allows or disallows the use of the PersistentVolumeClaim volume plugin"`
	// RBD allows or disallows the use of the RBD volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/rbd/README.md
	RBD bool `json:"rbd,omitempty" description:"allows or disallows the use of the RBD volume plugin"`
	// Cinder allows or disallows the use of the Cinder volume plugin.
	// More info: http://releases.k8s.io/HEAD/examples/mysql-cinder-pd/README.md
	Cinder bool `json:"cinder,omitempty" description:"allows or disallows the use of the Cinder volume plugin"`
	// CephFS allows or disallows the use of the CephFS volume plugin.
	CephFS bool `json:"cephfs,omitempty" description:"allows or disallows the use of the CephFS volume plugin"`
	// DownwardAPI allows or disallows the use of the DownwardAPI volume plugin.
	DownwardAPI bool `json:"downwardAPI,omitempty" description:"allows or disallows the use of the DownwardAPI volume plugin"`
	// FC allows or disallows the use of the FC volume plugin.
	FC bool `json:"fc,omitempty" description:"allows or disallows the use of the FC volume plugin"`
}

// SELinuxContextStrategyOptions defines the strategy type and any options used to create the strategy.
type SELinuxContextStrategyOptions struct {
	// Type is the strategy that will dictate what SELinux context is used in the SecurityContext.
	Type SELinuxContextStrategy `json:"type,omitempty" description:"strategy used to generate the SELinux context"`
	// seLinuxOptions required to run as; required for MustRunAs
	SELinuxOptions *kapi.SELinuxOptions `json:"seLinuxOptions,omitempty" description:"seLinuxOptions required to run as; required for MustRunAs"`
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
	Type RunAsUserStrategy `json:"type,omitempty" description:"strategy used to generate RunAsUser"`
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
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []PodSecurityPolicy `json:"items" description:"list of PodSecurityPolicy objects"`
}
