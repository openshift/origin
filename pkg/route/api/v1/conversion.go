package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	conversion "k8s.io/kubernetes/pkg/conversion"
	//oapi "github.com/openshift/origin/pkg/api"
	reflect "reflect"
	routeapi "github.com/openshift/origin/pkg/route/api"

	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			if len(obj.To.Kind) == 0 {
				obj.To.Kind = "Service"
			}
			if obj.SecondaryServices == nil {
				obj.SecondaryServices = make([]kapiv1.ObjectReference, 0)
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

	err = scheme.AddConversionFuncs(
		convert_api_RouteSpec_To_v1_RouteSpec,
		convert_v1_RouteSpec_To_api_RouteSpec,
	)
	if err != nil {
		panic(err)
	}

	//if err := scheme.AddFieldLabelConversionFunc("v1", "Route",
	//	oapi.GetFieldLabelConversionFunc(routeapi.RouteToSelectableFields(&routeapi.Route{}), nil),
	//); err != nil {
	//	panic(err)
	//}
}

func convert_api_RouteSpec_To_v1_RouteSpec(in *routeapi.RouteSpec, out *RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	for idx, inSvc := range in.To {
		if idx == 0 {
			if err := s.Convert(&in.To[0], &out.To, 0); err != nil {
				return err
			}
		} else {
			var outSvc kapiv1.ObjectReference
			if err := s.Convert(&inSvc, &outSvc, 0); err != nil {
				return err
			}
			out.SecondaryServices = append(out.SecondaryServices, outSvc)
		}
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1.RoutePort
	if in.Port != nil {
		out.Port = new(RoutePort)
		if err := s.Convert(in.Port, out.Port, 0); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1.TLSConfig
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

func convert_v1_RouteSpec_To_api_RouteSpec(in *RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	var toService kapi.ObjectReference
	if err := s.Convert(&in.To, &toService, 0); err != nil {
		return err
	}
	out.To = append(out.To, toService)
	for _, svc := range in.SecondaryServices {
		var outSvc kapi.ObjectReference
		if err := s.Convert(&svc, &outSvc, 0); err != nil {
			return err
		}

		out.To = append(out.To, outSvc)
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1.RoutePort
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := s.Convert(in.Port, out.Port, 0); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1.TLSConfig
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
