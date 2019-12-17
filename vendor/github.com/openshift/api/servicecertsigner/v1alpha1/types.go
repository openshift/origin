package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfig provides information to configure an operator to manage the service cert signing controllers
type ServiceCertSignerOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ServiceCertSignerOperatorConfigSpec   `json:"spec"`
	Status ServiceCertSignerOperatorConfigStatus `json:"status"`
}

type ServiceCertSignerOperatorConfigSpec struct {
	operatorv1.OperatorSpec `json:",inline"`
}

type ServiceCertSignerOperatorConfigStatus struct {
	operatorv1.OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfigList is a collection of items
type ServiceCertSignerOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the items
	Items []ServiceCertSignerOperatorConfig `json:"items"`
}
