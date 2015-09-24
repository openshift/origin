package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

type CatalogService struct {
	Target      kapi.ObjectReference `json:"target" description: "reference to a service"`
	Type        string               `json:"type" description:"the published service's claim type"`
	Description string               `json:"description" description:"user-defined description of the published service"`
}

type CatalogServiceList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []CatalogService `json:"items" description: "list of catalog services"`
}
