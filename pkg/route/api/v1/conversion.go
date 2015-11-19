package v1

import (
	"k8s.io/kubernetes/pkg/api"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

func init() {
	err := api.Scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			obj.To.Kind = "Service"
		},
	)
	if err != nil {
		panic(err)
	}

	err = api.Scheme.AddConversionFuncs()
	if err != nil {
		panic(err)
	}

	if err := api.Scheme.AddFieldLabelConversionFunc("v1", "Route",
		oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	); err != nil {
		panic(err)
	}
}
