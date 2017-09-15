package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func TestFieldSelectorConversions(t *testing.T) {
	converter := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(converter)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "Route",
		// Ensure all currently returned labels are supported
		routeapi.RouteToSelectableFields(&routeapi.Route{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"spec.host", "spec.path", "spec.to.name",
	)
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
