package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	"github.com/openshift/origin/pkg/route/apis/route"
)

func TestFieldSelectorConversions(t *testing.T) {
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{LegacySchemeBuilder.AddToScheme, route.LegacySchemeBuilder.AddToScheme},
		Kind:          LegacySchemeGroupVersion.WithKind("Route"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"spec.host", "spec.path", "spec.to.name"},
		FieldKeyEvaluatorFn:      route.RouteFieldSelector,
	}.Check(t)

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{SchemeBuilder.AddToScheme, route.SchemeBuilder.AddToScheme},
		Kind:          SchemeGroupVersion.WithKind("Route"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"spec.host", "spec.path", "spec.to.name"},
		FieldKeyEvaluatorFn:      route.RouteFieldSelector,
	}.Check(t)
}

func TestSupportingCamelConstants(t *testing.T) {
	scheme := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(scheme)

	for k, v := range map[TLSTerminationType]TLSTerminationType{
		"Reencrypt":   TLSTerminationReencrypt,
		"Edge":        TLSTerminationEdge,
		"Passthrough": TLSTerminationPassthrough,
	} {
		obj := &Route{
			Spec: RouteSpec{
				TLS: &TLSConfig{Termination: k},
			},
		}
		scheme.Default(obj)
		if obj.Spec.TLS.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, obj)
		}
	}
}
