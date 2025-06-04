package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=csisnapshotcontrollers,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/562
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=csi-snapshot-controller,operatorOrdering=01

// CSISnapshotController provides a means to configure an operator to manage the CSI snapshots. `cluster` is the canonical name.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type CSISnapshotController struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec CSISnapshotControllerSpec `json:"spec"`

	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status CSISnapshotControllerStatus `json:"status"`
}

// CSISnapshotControllerSpec is the specification of the desired behavior of the CSISnapshotController operator.
type CSISnapshotControllerSpec struct {
	OperatorSpec `json:",inline"`
}

// CSISnapshotControllerStatus defines the observed status of the CSISnapshotController operator.
type CSISnapshotControllerStatus struct {
	OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CSISnapshotControllerList contains a list of CSISnapshotControllers.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type CSISnapshotControllerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CSISnapshotController `json:"items"`
}
