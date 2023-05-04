package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineSet ensures that a specified number of machines replicas are running at any given time.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas",description="Desired Replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="Current Replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Ready Replicas"
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.availableReplicas",description="Observed number of available replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Machineset age"
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type MachineSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSetSpec   `json:"spec,omitempty"`
	Status MachineSetStatus `json:"status,omitempty"`
}

// MachineSetSpec defines the desired state of MachineSet
type MachineSetSpec struct {
	// Replicas is the number of desired replicas.
	// This is a pointer to distinguish between explicit zero and unspecified.
	// Defaults to 1.
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
	// MinReadySeconds is the minimum number of seconds for which a newly created machine should be ready.
	// Defaults to 0 (machine will be considered available as soon as it is ready)
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`
	// DeletePolicy defines the policy used to identify nodes to delete when downscaling.
	// Defaults to "Random".  Valid values are "Random, "Newest", "Oldest"
	// +kubebuilder:validation:Enum=Random;Newest;Oldest
	DeletePolicy string `json:"deletePolicy,omitempty"`
	// Selector is a label query over machines that should match the replica count.
	// Label keys and values that must match in order to be controlled by this MachineSet.
	// It must match the machine template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector metav1.LabelSelector `json:"selector"`
	// Template is the object that describes the machine that will be created if
	// insufficient replicas are detected.
	// +optional
	Template MachineTemplateSpec `json:"template,omitempty"`
}

// MachineSetDeletePolicy defines how priority is assigned to nodes to delete when
// downscaling a MachineSet. Defaults to "Random".
type MachineSetDeletePolicy string

const (
	// RandomMachineSetDeletePolicy prioritizes both Machines that have the annotation
	// "cluster.k8s.io/delete-machine=yes" and Machines that are unhealthy
	// (Status.ErrorReason or Status.ErrorMessage are set to a non-empty value).
	// Finally, it picks Machines at random to delete.
	RandomMachineSetDeletePolicy MachineSetDeletePolicy = "Random"
	// NewestMachineSetDeletePolicy prioritizes both Machines that have the annotation
	// "cluster.k8s.io/delete-machine=yes" and Machines that are unhealthy
	// (Status.ErrorReason or Status.ErrorMessage are set to a non-empty value).
	// It then prioritizes the newest Machines for deletion based on the Machine's CreationTimestamp.
	NewestMachineSetDeletePolicy MachineSetDeletePolicy = "Newest"
	// OldestMachineSetDeletePolicy prioritizes both Machines that have the annotation
	// "cluster.k8s.io/delete-machine=yes" and Machines that are unhealthy
	// (Status.ErrorReason or Status.ErrorMessage are set to a non-empty value).
	// It then prioritizes the oldest Machines for deletion based on the Machine's CreationTimestamp.
	OldestMachineSetDeletePolicy MachineSetDeletePolicy = "Oldest"
)

// MachineTemplateSpec describes the data needed to create a Machine from a template
type MachineTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the machine.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec MachineSpec `json:"spec,omitempty"`
}

// MachineSetStatus defines the observed state of MachineSet
type MachineSetStatus struct {
	// Replicas is the most recently observed number of replicas.
	Replicas int32 `json:"replicas"`
	// The number of replicas that have labels matching the labels of the machine template of the MachineSet.
	// +optional
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas,omitempty"`
	// The number of ready replicas for this MachineSet. A machine is considered ready when the node has been created and is "Ready".
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// The number of available replicas (ready for at least minReadySeconds) for this MachineSet.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
	// ObservedGeneration reflects the generation of the most recently observed MachineSet.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// In the event that there is a terminal problem reconciling the
	// replicas, both ErrorReason and ErrorMessage will be set. ErrorReason
	// will be populated with a succinct value suitable for machine
	// interpretation, while ErrorMessage will contain a more verbose
	// string suitable for logging and human consumption.
	//
	// These fields should not be set for transitive errors that a
	// controller faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the MachineTemplate's spec or the configuration of
	// the machine controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the machine controller, or the
	// responsible machine controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the MachineSet object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *MachineSetStatusError `json:"errorReason,omitempty"`
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineSetList contains a list of MachineSet
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type MachineSetList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MachineSet `json:"items"`
}
