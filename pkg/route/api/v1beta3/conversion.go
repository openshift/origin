package v1beta3

import (
	"fmt"

	"k8s.io/kubernetes/pkg/runtime"
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

	// Add field conversion funcs.
	err = scheme.AddFieldLabelConversionFunc("v1beta3", "Route",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"spec.host",
				"spec.path",
				"spec.to.name":
				return label, value, nil
				// This is for backwards compatibility with old v1 clients which send spec.host
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

}
