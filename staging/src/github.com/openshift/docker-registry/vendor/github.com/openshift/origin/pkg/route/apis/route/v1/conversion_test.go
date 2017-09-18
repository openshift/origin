package v1_test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Route",
		// Ensure all currently returned labels are supported
		routeapi.RouteToSelectableFields(&routeapi.Route{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"spec.host", "spec.path", "spec.to.name",
	)
}

func TestSupportingCamelConstants(t *testing.T) {
	for k, v := range map[routeapiv1.TLSTerminationType]routeapiv1.TLSTerminationType{
		"Reencrypt":   routeapiv1.TLSTerminationReencrypt,
		"Edge":        routeapiv1.TLSTerminationEdge,
		"Passthrough": routeapiv1.TLSTerminationPassthrough,
	} {
		obj := &routeapiv1.Route{
			Spec: routeapiv1.RouteSpec{
				TLS: &routeapiv1.TLSConfig{Termination: k},
			},
		}
		kapi.Scheme.Default(obj)
		if obj.Spec.TLS.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, obj)
		}
	}
}
