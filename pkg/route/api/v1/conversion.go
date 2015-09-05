package v1

import (
	"k8s.io/kubernetes/pkg/api"
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
}
