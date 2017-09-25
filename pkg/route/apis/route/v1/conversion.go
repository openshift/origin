package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Route",
		oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	); err != nil {
		return err
	}
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Route",
		oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	)
}
