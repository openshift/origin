package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=pinnedimagesets,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1713
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=PinnedImages
// +kubebuilder:metadata:labels=openshift.io/operator-managed=

// PinnedImageSet describes a set of images that should be pinned by CRI-O and
// pulled to the nodes which are members of the declared MachineConfigPools.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type PinnedImageSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of this pinned image set.
	// +required
	Spec PinnedImageSetSpec `json:"spec"`

	// status describes the last observed state of this pinned image set.
	// +optional
	Status PinnedImageSetStatus `json:"status"`
}

// PinnedImageSetStatus describes the current state of a PinnedImageSet.
type PinnedImageSetStatus struct {
	// conditions represent the observations of a pinned image set's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
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
	// These image references should all be by digest, tags aren't allowed.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=500
	// +listType=map
	// +listMapKey=name
	PinnedImages []PinnedImageRef `json:"pinnedImages"`
}

type PinnedImageRef struct {
	// name is an OCI Image referenced by digest.
	//
	// The format of the image ref is:
	// host[:port][/namespace]/name@sha256:<digest>
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$')`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +kubebuilder:validation:XValidation:rule=`self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$')`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PinnedImageSetList is a list of PinnedImageSet resources
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type PinnedImageSetList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []PinnedImageSet `json:"items"`
}
