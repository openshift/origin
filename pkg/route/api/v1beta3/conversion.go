package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	err := api.Scheme.AddDefaultingFuncs(
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

	err = api.Scheme.AddConversionFuncs()
	if err != nil {
		panic(err)
	}
}
