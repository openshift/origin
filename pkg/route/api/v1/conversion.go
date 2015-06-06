package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/route/api"
)

func convert_v1_Route_To_api_Route(in *Route, out *newer.Route, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	out.Path = in.Spec.Path
	out.Host = in.Spec.Host
	if in.Spec.To.Kind == "Service" || len(in.Spec.To.Kind) == 0 {
		out.ServiceName = in.Spec.To.Name
	}
	return s.Convert(&in.Spec.TLS, &out.TLS, 0)
}

func convert_api_Route_To_v1_Route(in *newer.Route, out *Route, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	out.Spec.Path = in.Path
	out.Spec.Host = in.Host
	out.Spec.To.Kind = "Service"
	out.Spec.To.Name = in.ServiceName
	return s.Convert(&in.TLS, &out.Spec.TLS, 0)
}

func init() {
	err := api.Scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			obj.To.Kind = "Service"
		},
	)
	if err != nil {
		panic(err)
	}

	err = api.Scheme.AddConversionFuncs(
		convert_v1_Route_To_api_Route,
		convert_api_Route_To_v1_Route,
	)
	if err != nil {
		panic(err)
	}
}
