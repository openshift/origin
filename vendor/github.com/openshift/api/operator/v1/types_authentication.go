package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Authentication provides information to configure an operator to manage authentication.
type Authentication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	// +required
	Spec AuthenticationSpec `json:"spec,omitempty"`
	// +optional
	Status AuthenticationStatus `json:"status,omitempty"`
}

type AuthenticationSpec struct {
	OperatorSpec `json:",inline"`
}

type AuthenticationStatus struct {
	OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthenticationList is a collection of items
type AuthenticationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Authentication `json:"items"`
}
