package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MyOtherOperatorResource is an example operator configuration type
type MyOtherOperatorResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Required
	// +required
	Spec MyOtherOperatorResourceSpec `json:"spec"`
}

type MyOtherOperatorResourceSpec struct {
	Name            string `json:"name"`
	DeprecatedField string `json:"deprecatedField"`
}
