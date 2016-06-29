package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	if err := scheme.AddConversionFuncs(); err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "Route",
		oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	); err != nil {
		panic(err)
	}
}
