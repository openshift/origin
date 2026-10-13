package resourceread

import (
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	routeScheme = runtime.NewScheme()
	routeCodecs = serializer.NewCodecFactory(routeScheme)
)

func init() {
	if err := routev1.AddToScheme(routeScheme); err != nil {
		panic(err)
	}
}

func ReadRouteV1OrDie(objBytes []byte) *routev1.Route {
	requiredObj, err := runtime.Decode(routeCodecs.UniversalDecoder(routev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*routev1.Route)
}
