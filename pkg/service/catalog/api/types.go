package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// CatalogService stores information about published service
type CatalogService struct {
	kapi.TypeMeta
	kapi.ObjectMeta
	// Target is a reference to a service
	Target kapi.ObjectReference
	// ClaimType is a published service's claim type
	ClaimType string
	// Description is a user-defined description of the published service
	Description string
}

// CatalogServiceList is a list of CatalogService objects
type CatalogServiceList struct {
	kapi.TypeMeta
	kapi.ListMeta
	// Items contains a list of catalog services
	Items []CatalogService
}
