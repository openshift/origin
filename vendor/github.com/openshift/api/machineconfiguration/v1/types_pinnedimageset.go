package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=pinnedimagesets,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2198
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=PinnedImages
// +kubebuilder:metadata:labels=openshift.io/operator-managed=

// PinnedImageSet describes a set of images that should be pinned by CRI-O and
// pulled to the nodes which are members of the declared MachineConfigPools.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type PinnedImageSet struct {
	metav1.TypeMeta `json:",inline"`
	
	// metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of this pinned image set.
	// +required
	Spec PinnedImageSetSpec `json:"spec"`
}

// PinnedImageSetSpec defines the desired state of a PinnedImageSet.
type PinnedImageSetSpec struct {
	// pinnedImages is a list of OCI Image referenced by digest that should be
	// pinned and pre-loaded by the nodes of a MachineConfigPool.
	// Translates into a new file inside the /etc/crio/crio.conf.d directory
	// with content similar to this:
	//
	//      pinned_images = [
	//              "quay.io/openshift-release-dev/ocp-release@sha256:...",
	//              "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...",
	//              "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...",
	//              ...
	//      ]
	//
	// Image references must be by digest.
	// A maximum of 500 images may be specified.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=500
	// +listType=map
	// +listMapKey=name
	PinnedImages []PinnedImageRef `json:"pinnedImages"`
}

// PinnedImageRef represents a reference to an OCI image
type PinnedImageRef struct {
	// name is an OCI Image referenced by digest.
	// The format of the image pull spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// +required
	Name ImageDigestFormat `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PinnedImageSetList is a list of PinnedImageSet resources
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type PinnedImageSetList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items contains a collection of PinnedImageSet resources.
	// +kubebuilder:validation:MaxItems=500
	// +optional
	Items []PinnedImageSet `json:"items"`
}
