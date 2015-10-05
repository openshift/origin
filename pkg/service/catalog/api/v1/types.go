package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// CatalogService stores information about published service
type CatalogService struct {
	kapi.TypeMeta
	kapi.ObjectMeta
	// Target is a reference to a service
	Target kapi.ObjectReference `json:"target" description: "reference to a service"`
	// ClaimType is a published service's claim type
	ClaimType string `json:"claimtype" description:"the published service's claim type"`
	// Description is a user-defined description of the published service
	Description string `json:"description" description:"user-defined description of the published service"`
}

// CatalogServiceList is a list of CatalogService objects
type CatalogServiceList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	// Items contains a list of catalog services
	Items []CatalogService `json:"items" description: "list of catalog services"`
}
