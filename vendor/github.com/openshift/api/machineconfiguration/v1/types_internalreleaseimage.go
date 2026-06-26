package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=internalreleaseimages,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2510
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=NoRegistryClusterInstall
// +kubebuilder:metadata:labels=openshift.io/operator-managed=
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="internalreleaseimage is a singleton, .metadata.name must be 'cluster'"

// InternalReleaseImage is used to keep track and manage a set
// of release bundles (OCP and OLM operators images) that are stored
// into the control planes nodes.
// This is a singleton resource with 'cluster' as the only valid name. 
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type InternalReleaseImage struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of this internal release image.
	// +required
	Spec InternalReleaseImageSpec `json:"spec,omitzero"`

	// status describes the last observed state of this internal release image.
	// +optional
	Status InternalReleaseImageStatus `json:"status,omitzero"`
}

// InternalReleaseImageSpec defines the desired state of a InternalReleaseImage.
type InternalReleaseImageSpec struct {
	// releases is a list of release bundle identifiers that the user wants to
	// add/remove to/from the control plane nodes.
	// Entries must be unique, keyed on the name field.
	// releases must contain at least one entry and must not exceed 16 entries.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +listType=map
	// +listMapKey=name
	// +required
	Releases []InternalReleaseImageRef `json:"releases,omitempty"`
}

// InternalReleaseImageRef is used to provide a simple reference for a release
// bundle. Currently it contains only the name field.
type InternalReleaseImageRef struct {
	// name indicates the desired release bundle identifier. This field is required and must be between 1 and 64 characters long.
	// The expected name format is ocp-release-bundle-<version>-<arch|stream>.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule=`self.matches('^ocp-release-bundle-[0-9]+\\.[0-9]+\\.[0-9]+-[A-Za-z0-9._-]+$')`,message="must be ocp-release-bundle-<version>-<arch|stream> and <= 64 chars"
	Name string `json:"name,omitempty"`
}

// InternalReleaseImageStatus describes the current state of a InternalReleaseImage.
type InternalReleaseImageStatus struct {
	// conditions represent the observations of the InternalReleaseImage controller current state.
	// Valid types are: Degraded.
	// If Degraded is true, that means something has gone wrong in the controller.
	// The conditions list must contain at most 5 entries.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// releases is a list of the release bundles currently owned and managed by the
	// cluster.
	// A release bundle content could be safely pulled only when its Conditions field
	// contains at least an Available entry set to "True" and Degraded to "False".
	// Entries must be unique, keyed on the name field.
	// releases must contain at least one entry and must not exceed 32 entries.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +required
	Releases []InternalReleaseImageBundleStatus `json:"releases,omitempty"`
}

// InternalReleaseImageStatusConditionType describes the possible states for InternalReleaseImageStatus.
// +enum
type InternalReleaseImageStatusConditionType string

const (
	// InternalReleaseImageStatusConditionTypeDegraded describes a failure in the controller.
	InternalReleaseImageStatusConditionTypeDegraded InternalReleaseImageStatusConditionType = "Degraded"
)

// InternalReleaseImageBundleStatus describes the observed state of a single release bundle managed by the cluster.
type InternalReleaseImageBundleStatus struct {
	// conditions represent the observations of an internal release image current state. Valid types are:
	// Mounted, Installing, Available, Removing and Degraded.
	//
	// If Mounted is true, that means that a valid ISO has been discovered and mounted on one of the cluster nodes.
	// If Installing is true, that means that a new release bundle is currently being copied on one (or more) cluster nodes, and not yet completed.
	// If Available is true, it means that the release has been previously installed on all the cluster nodes, and it can be used.
	// If Removing is true, it means that a release deletion is in progress on one (or more) cluster nodes, and not yet completed.
	// If Degraded is true, that means something has gone wrong (possibly on one or more cluster nodes).
	//
	// In general, after installing a new release bundle, it is required to wait for the Conditions "Available" to become "True" (and all
	// the other conditions to be equal to "False") before being able to pull its content.
	// When present, conditions must contain at least 1 entry and must not exceed 5 entries.
	//
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// name indicates the desired release bundle identifier. This field is required and must be between 1 and 64 characters long.
	// The expected name format is ocp-release-bundle-<version>-<arch|stream>.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule=`self.matches('^ocp-release-bundle-[0-9]+\\.[0-9]+\\.[0-9]+-[A-Za-z0-9._-]+$')`,message="must be ocp-release-bundle-<version>-<arch|stream> and <= 64 chars"
	// +required
	Name string `json:"name,omitempty"`
	// image is an OCP release image referenced by digest.
	// The format of the image pull spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// The field is optional, and it will be provided after a release has been successfully installed.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
	// +optional
	Image string `json:"image,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InternalReleaseImageList is a list of InternalReleaseImage resources
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type InternalReleaseImageList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []InternalReleaseImage `json:"items"`
}
