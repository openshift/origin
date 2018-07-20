package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/route/apis/route"
	routev1helpers "github.com/openshift/origin/pkg/route/apis/route/v1"
	"k8s.io/kubernetes/pkg/apis/core"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"
)

// InstallLegacyRoute this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyRoute(scheme *runtime.Scheme) {
	InstallExternalLegacyRoute(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalRouteTypes,
		core.AddToScheme,
		corev1conversions.AddToScheme,

		addLegacyRouteFieldSelectorKeyConversions,
		routev1helpers.RegisterDefaults,
		routev1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

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

func addUngroupifiedInternalRouteTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&route.Route{},
		&route.RouteList{},
	)
	return nil
}

func addLegacyRouteFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(GroupVersion.String(), "Route", legacyRouteFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

func legacyRouteFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "spec.path",
		"spec.host",
		"spec.to.name":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
