package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCatalogAPIServer provides information to configure an operator to manage Service Catalog API Server
type ServiceCatalogAPIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceCatalogAPIServerSpec   `json:"spec,omitempty"`
	Status ServiceCatalogAPIServerStatus `json:"status,omitempty"`
}

type ServiceCatalogAPIServerSpec struct {
	OperatorSpec `json:",inline"`
}

type ServiceCatalogAPIServerStatus struct {
	OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCatalogAPIServerList is a collection of items
type ServiceCatalogAPIServerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items contains the items
	Items []ServiceCatalogAPIServer `json:"items"`
}
