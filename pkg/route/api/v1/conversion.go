package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			obj.To.Kind = "Service"
		},
		func(obj *TLSConfig) {
			if len(obj.Termination) == 0 && len(obj.DestinationCACertificate) == 0 {
				obj.Termination = TLSTerminationEdge
			}
			switch obj.Termination {
			case TLSTerminationType("Reencrypt"):
				obj.Termination = TLSTerminationReencrypt
			case TLSTerminationType("Edge"):
				obj.Termination = TLSTerminationEdge
			case TLSTerminationType("Passthrough"):
				obj.Termination = TLSTerminationPassthrough
			}
		},
	)
	if err != nil {
		panic(err)
	}

	err = scheme.AddConversionFuncs()
	if err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "Route",
		oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	); err != nil {
		panic(err)
	}
}
