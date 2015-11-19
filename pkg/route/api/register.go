package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Route{},
		&RouteList{},
	)
}

func (*Route) IsAnAPIObject()     {}
func (*RouteList) IsAnAPIObject() {}
