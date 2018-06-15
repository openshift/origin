package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateServiceBrokerConfig holds information related to the template
// service broker
type TemplateServiceBrokerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// TemplateNamespaces indicates the namespace(s) in which the template service
	// broker looks for templates to serve to the catalog.
	TemplateNamespaces []string `json:"templateNamespaces"`
}
