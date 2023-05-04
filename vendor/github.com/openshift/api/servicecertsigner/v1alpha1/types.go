package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfig provides information to configure an operator to manage the service cert signing controllers
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type ServiceCertSignerOperatorConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
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
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type ServiceCertSignerOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the items
	Items []ServiceCertSignerOperatorConfig `json:"items"`
}
