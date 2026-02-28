package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OSImageStream describes a set of streams and associated images available
// for the MachineConfigPools to be used as base OS images.
//
// The resource is a singleton named "cluster".
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=osimagestreams,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2555
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=OSStreams
// +kubebuilder:metadata:labels=openshift.io/operator-managed=
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="osimagestream is a singleton, .metadata.name must be 'cluster'"
type OSImageStream struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec contains the desired OSImageStream config configuration.
	// +required
	Spec *OSImageStreamSpec `json:"spec,omitempty"`

	// status describes the last observed state of this OSImageStream.
	// Populated by the MachineConfigOperator after reading release metadata.
	// When not present, the controller has not yet reconciled this resource.
	// +optional
	Status OSImageStreamStatus `json:"status,omitempty,omitzero"`
}

// OSImageStreamStatus describes the current state of a OSImageStream
// +kubebuilder:validation:XValidation:rule="self.defaultStream in self.availableStreams.map(s, s.name)",message="defaultStream must reference a stream name from availableStreams"
type OSImageStreamStatus struct {

	// availableStreams is a list of the available OS Image Streams that can be
	// used as the base image for MachineConfigPools.
	// availableStreams is required, must have at least one item, must not exceed
	// 100 items, and must have unique entries keyed on the name field.
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=map
	// +listMapKey=name
	AvailableStreams []OSImageStreamSet `json:"availableStreams,omitempty"`

	// defaultStream is the name of the stream that should be used as the default
	// when no specific stream is requested by a MachineConfigPool.
	//
	// It must be a valid RFC 1123 subdomain between 1 and 253 characters in length,
	// consisting of lowercase alphanumeric characters, hyphens ('-'), and periods ('.'),
	// and must reference the name of one of the streams in availableStreams.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	DefaultStream string `json:"defaultStream,omitempty"`
}

// OSImageStreamSpec defines the desired state of a OSImageStream.
type OSImageStreamSpec struct {
	// defaultStream is the desired name of the stream that should be used as the
	// default when no specific stream is requested by a MachineConfigPool.
	//
	// This field is set by the installer during installation. Users may need to
	// update it if the currently selected stream is no longer available, for
	// example when the stream has reached its End of Life.
	// The MachineConfigOperator uses this value to determine which stream from
	// status.availableStreams to apply as the default for MachineConfigPools
	// that do not specify a stream override.
	//
	// It must be a valid RFC 1123 subdomain between 1 and 253 characters in length,
	// consisting of lowercase alphanumeric characters, hyphens ('-'), and periods ('.').
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	DefaultStream string `json:"defaultStream,omitempty"`
}

type OSImageStreamSet struct {
	// name is the required identifier of the stream.
	//
	// name is determined by the operator based on the OCI label of the
	// discovered OS or Extension Image.
	//
	// Must be a valid RFC 1123 subdomain between 1 and 253 characters in length,
	// consisting of lowercase alphanumeric characters, hyphens ('-'), and periods ('.').
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	Name string `json:"name,omitempty"`

	// osImage is a required OS Image referenced by digest.
	//
	// osImage contains the immutable, fundamental operating system components, including the kernel
	// and base utilities, that define the core environment for the node's host operating system.
	//
	// The format of the image pull spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// +required
	OSImage ImageDigestFormat `json:"osImage,omitempty"`

	// osExtensionsImage is a required OS Extensions Image referenced by digest.
	//
	// osExtensionsImage bundles the extra repositories used to enable extensions, augmenting
	// the base operating system without modifying the underlying immutable osImage.
	//
	// The format of the image pull spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// +required
	OSExtensionsImage ImageDigestFormat `json:"osExtensionsImage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OSImageStreamList is a list of OSImageStream resources
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type OSImageStreamList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []OSImageStream `json:"items"`
}
