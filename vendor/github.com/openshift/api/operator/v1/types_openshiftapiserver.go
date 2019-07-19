package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenShiftAPIServer provides information to configure an operator to manage openshift-apiserver.
type OpenShiftAPIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Required
	// +required
	Spec OpenShiftAPIServerSpec `json:"spec"`
	// +optional
	Status OpenShiftAPIServerStatus `json:"status"`
}

type OpenShiftAPIServerSpec struct {
	OperatorSpec `json:",inline"`
}

type OpenShiftAPIServerStatus struct {
	OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenShiftAPIServerList is a collection of items
type OpenShiftAPIServerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	// Items contains the items
	Items []OpenShiftAPIServer `json:"items"`
}
