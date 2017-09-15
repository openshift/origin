package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc("v1", "Route",
		apihelpers.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	)
}
