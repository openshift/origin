package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

type CatalogService struct {
	// Target is a reference to a service
	Target kapi.ObjectReference
	// Type is a published service's claim type
	Type string
	// Description is a user-defined description of the published service
	Description string
}

type CatalogServiceList struct {
	kapi.TypeMeta
	kapi.ListMeta
	// Items contains a list of catalog services
	Items []CatalogService
}
