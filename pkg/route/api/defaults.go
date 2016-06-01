
package api

import (
	"k8s.io/kubernetes/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
)

func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *RouteSpec) {
			if obj.AdditionalTos == nil {
				obj.AdditionalTos = make([]kapi.ObjectReference, 0)
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
