package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Route{},
		&RouteList{},
	)
}

func (*Route) IsAnAPIObject()     {}
func (*RouteList) IsAnAPIObject() {}
