package v1

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&Route{},
		&RouteList{},
	)
}

func (*Route) IsAnAPIObject()     {}
func (*RouteList) IsAnAPIObject() {}
