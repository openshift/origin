package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	routev1 "github.com/openshift/api/route/v1"
)

func InstallExternalLegacyRoute(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedRouteTypes,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedRouteTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&routev1.Route{},
		&routev1.RouteList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
