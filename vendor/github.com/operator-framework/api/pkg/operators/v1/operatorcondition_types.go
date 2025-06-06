package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Upgradeable indicates that the operator is upgradeable
	Upgradeable string = "Upgradeable"
)

// OperatorConditionSpec allows a cluster admin to convey information about the state of an operator to OLM, potentially overriding state reported by the operator.
type OperatorConditionSpec struct {
	ServiceAccounts []string           `json:"serviceAccounts,omitempty"`
	Deployments     []string           `json:"deployments,omitempty"`
	Overrides       []metav1.Condition `json:"overrides,omitempty"`
}

// OperatorConditionStatus allows an operator to convey information its state to OLM. The status may trail the actual
// state of a system.
type OperatorConditionStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:resource:shortName=condition,categories=olm
// +kubebuilder:subresource:status
// OperatorCondition is a Custom Resource of type `OperatorCondition` which is used to convey information to OLM about the state of an operator.
type OperatorCondition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   OperatorConditionSpec   `json:"spec,omitempty"`
	Status OperatorConditionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// OperatorConditionList represents a list of Conditions.
type OperatorConditionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []OperatorCondition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperatorCondition{}, &OperatorConditionList{})
}
