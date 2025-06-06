package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Upgradeable indicates that the operator is upgradeable
	Upgradeable string = "Upgradeable"
)

// ConditionType codifies a condition's type.
type ConditionType string

// OperatorConditionSpec allows an operator to report state to OLM and provides
// cluster admin with the ability to manually override state reported by the operator.
type OperatorConditionSpec struct {
	ServiceAccounts []string           `json:"serviceAccounts,omitempty"`
	Deployments     []string           `json:"deployments,omitempty"`
	Overrides       []metav1.Condition `json:"overrides,omitempty"`
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
}

// OperatorConditionStatus allows OLM to convey which conditions have been observed.
type OperatorConditionStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:storageversion
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
