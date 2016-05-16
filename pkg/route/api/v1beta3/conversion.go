package v1beta3

import (
	"fmt"

	conversion "k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	routeapi "github.com/openshift/origin/pkg/route/api"

	kapi "k8s.io/kubernetes/pkg/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			if len(obj.To.Kind) == 0 {
				obj.To.Kind = "Service"
			}
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

	err = scheme.AddConversionFuncs(convert_api_RouteSpec_To_v1beta3_RouteSpec,
				convert_v1beta3_RouteSpec_To_api_RouteSpec,
			)
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

func convert_api_RouteSpec_To_v1beta3_RouteSpec(in *routeapi.RouteSpec, out *RouteSpec, s conversion.Scope) error {
	out.Host = in.Host
	out.Path = in.Path
	if err := s.Convert(&in.To[0], &out.To, 0); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1beta3.RoutePort
	if in.Port != nil {
		out.Port = new(RoutePort)
		if err := s.Convert(in.Port, out.Port, 0); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1beta3.TLSConfig
	if in.TLS != nil {
		out.TLS = new(TLSConfig)
		if err := s.Convert(in.TLS, out.TLS, 0); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func convert_v1beta3_RouteSpec_To_api_RouteSpec(in *RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	out.Host = in.Host
	out.Path = in.Path
	out.To = make([]kapi.ObjectReference, 1)
	if err := s.Convert(&in.To, &out.To[0], 0); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1beta3.RoutePort
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := s.Convert(in.Port, out.Port, 0); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1beta3.TLSConfig
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := s.Convert(in.TLS, out.TLS, 0); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}
