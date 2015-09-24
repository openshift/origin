package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&CatalogService{},
		&CatalogServiceList{},
	)
}

func (*CatalogService) IsAnAPIObject()     {}
func (*CatalogServiceList) IsAnAPIObject() {}
