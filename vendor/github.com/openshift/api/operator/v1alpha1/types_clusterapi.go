package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=clusterapis,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2564
// +openshift:file-pattern=cvoRunLevel=0000_30,operatorName=cluster-api,operatorOrdering=01
// +openshift:enable:FeatureGate=ClusterAPIMachineManagement
// +kubebuilder:metadata:annotations="release.openshift.io/feature-gate=ClusterAPIMachineManagement"

// ClusterAPI provides configuration for the capi-operator.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="clusterapi is a singleton, .metadata.name must be 'cluster'"
type ClusterAPI struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the capi-operator.
	// +required
	Spec *ClusterAPISpec `json:"spec,omitempty"`

	// status defines the observed status of the capi-operator.
	// +optional
	Status ClusterAPIStatus `json:"status,omitzero"`
}

// ClusterAPISpec defines the desired configuration of the capi-operator.
// The spec is required but we deliberately allow it to be empty.
// +kubebuilder:validation:MinProperties=0
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.unmanagedCustomResourceDefinitions) || has(self.unmanagedCustomResourceDefinitions)",message="unmanagedCustomResourceDefinitions cannot be unset once set"
type ClusterAPISpec struct {
	// unmanagedCustomResourceDefinitions is a list of ClusterResourceDefinition (CRD)
	// names that should not be managed by the capi-operator installer
	// controller. This allows external actors to own specific CRDs while
	// capi-operator manages others.
	//
	// Each CRD name must be a valid DNS-1123 subdomain consisting of lowercase
	// alphanumeric characters, '-' or '.', and must start and end with an
	// alphanumeric character, with a maximum length of 253 characters.
	// CRD names must contain at least two '.' characters.
	// Example: "clusters.cluster.x-k8s.io"
	//
	// Items cannot be removed from this list once added.
	//
	// The maximum number of unmanagedCustomResourceDefinitions is 128.
	//
	// +optional
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(item, item in self)",message="items cannot be removed from unmanagedCustomResourceDefinitions list"
	// +kubebuilder:validation:items:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +kubebuilder:validation:items:XValidation:rule="self.split('.').size() > 2",message="CRD names must contain at least two '.' characters."
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	UnmanagedCustomResourceDefinitions []string `json:"unmanagedCustomResourceDefinitions,omitempty"`
}

// RevisionName represents the name of a revision. The name must be between 1
// and 255 characters long.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=255
type RevisionName string

// ClusterAPIStatus describes the current state of the capi-operator.
// +kubebuilder:validation:XValidation:rule="self.revisions.exists(r, r.name == self.desiredRevision && self.revisions.all(s, s.revision <= r.revision))",message="desiredRevision must be the name of the revision with the highest revision number"
// +kubebuilder:validation:XValidation:rule="!has(self.currentRevision) || self.revisions.exists(r, r.name == self.currentRevision)",message="currentRevision must correspond to an entry in the revisions list"
type ClusterAPIStatus struct {
	// currentRevision is the name of the most recently fully applied revision.
	// It is written by the installer controller. If it is absent, it indicates
	// that no revision has been fully applied yet.
	// If set, currentRevision must correspond to an entry in the revisions list.
	// +optional
	CurrentRevision RevisionName `json:"currentRevision,omitempty"`

	// desiredRevision is the name of the desired revision. It is written by the
	// revision controller. It must be set to the name of the entry in the
	// revisions list with the highest revision number.
	// +required
	DesiredRevision RevisionName `json:"desiredRevision,omitempty"`

	// revisions is a list of all currently active revisions. A revision is
	// active until the installer controller updates currentRevision to a later
	// revision. It is written by the revision controller.
	//
	// The maximum number of revisions is 16.
	// All revisions must have a unique name.
	// All revisions must have a unique revision number.
	// When adding a revision, the revision number must be greater than the highest revision number in the list.
	// Revisions are immutable, although they can be deleted.
	//
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.exists_one(y, x.name == y.name))",message="each revision must have a unique name"
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.exists_one(y, x.revision == y.revision))",message="each revision must have a unique revision number"
	// +kubebuilder:validation:XValidation:rule="self.all(new, oldSelf.exists(old, old.name == new.name) || oldSelf.all(old, new.revision > old.revision))",message="new revisions must have a revision number greater than all existing revisions"
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(old, !self.exists(new, new.name == old.name) || self.exists(new, new == old))",message="existing revisions are immutable, but may be removed"
	Revisions []ClusterAPIInstallerRevision `json:"revisions,omitempty"`
}

// +structType=atomic
type ClusterAPIInstallerRevision struct {
	// name is the name of a revision.
	// +required
	Name RevisionName `json:"name,omitempty"`

	// revision is a monotonically increasing number that is assigned to a revision.
	// +required
	// +kubebuilder:validation:Minimum=1
	Revision int64 `json:"revision,omitempty"`

	// contentID uniquely identifies the content of this revision.
	// The contentID must be between 1 and 255 characters long.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	ContentID string `json:"contentID,omitempty"`

	// unmanagedCustomResourceDefinitions is a list of the names of
	// ClusterResourceDefinition (CRD) objects which are included in this
	// revision, but which should not be installed or updated. If not set, all
	// CRDs in the revision will be managed by the CAPI operator.
	// +listType=atomic
	// +kubebuilder:validation:items:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	UnmanagedCustomResourceDefinitions []string `json:"unmanagedCustomResourceDefinitions,omitempty"`

	// components is a list of components which will be installed by this
	// revision. Components will be installed in the order they are listed. If
	// omitted no components will be installed.
	//
	// The maximum number of components is 32.
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	Components []ClusterAPIInstallerComponent `json:"components,omitempty"`
}

// InstallerComponentType is the type of component to install.
// +kubebuilder:validation:Enum=Image
// +enum
type InstallerComponentType string

const (
	// InstallerComponentTypeImage is an image source for a component.
	InstallerComponentTypeImage InstallerComponentType = "Image"
)

// ClusterAPIInstallerComponent defines a component which will be installed by this revision.
// +union
// +kubebuilder:validation:XValidation:rule="self.type == 'Image' ? has(self.image) : !has(self.image)",message="image is required when type is Image, and forbidden otherwise"
type ClusterAPIInstallerComponent struct {
	// type is the source type of the component.
	// The only valid value is Image.
	// When set to Image, the image field must be set and will define an image source for the component.
	// +required
	// +unionDiscriminator
	Type InstallerComponentType `json:"type,omitempty"`

	// image defines an image source for a component. The image must contain a
	// /capi-operator-installer directory containing the component manifests.
	// +optional
	Image ClusterAPIInstallerComponentImage `json:"image,omitzero"`
}

// ImageDigestFormat is a type that conforms to the format host[:port][/namespace]/name@sha256:<digest>.
// The digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
// The length of the field must be between 1 to 447 characters.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=447
// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
type ImageDigestFormat string

// ClusterAPIInstallerComponentImage defines an image source for a component.
type ClusterAPIInstallerComponentImage struct {
	// ref is an image reference to the image containing the component manifests. The reference
	// must be a valid image digest reference in the format host[:port][/namespace]/name@sha256:<digest>.
	// The digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the field must be between 1 to 447 characters.
	// +required
	Ref ImageDigestFormat `json:"ref,omitempty"`

	// profile is the name of a profile to use from the image.
	//
	// A profile name may be up to 255 characters long. It must consist of alphanumeric characters, '-', or '_'.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self.matches('^[a-zA-Z0-9-_]+$')",message="profile must consist of alphanumeric characters, '-', or '_'"
	Profile string `json:"profile,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAPIList contains a list of ClusterAPI configurations
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type ClusterAPIList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// items contains the items
	Items []ClusterAPI `json:"items"`
}
