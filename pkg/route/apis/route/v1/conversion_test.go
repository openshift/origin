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
	for k, v := range map[routeapiv1.TLSTerminationType]routeapi.TLSTerminationType{
		"Reencrypt":   routeapi.TLSTerminationReencrypt,
		"Edge":        routeapi.TLSTerminationEdge,
		"Passthrough": routeapi.TLSTerminationPassthrough,
	} {
		obj := &routeapiv1.TLSConfig{Termination: k}
		out := &routeapi.TLSConfig{}
		if err := kapi.Scheme.Convert(obj, out, nil); err != nil {
			t.Errorf("%s: did not convert: %v", k, err)
			continue
		}
		if out.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, out)
		}
	}
}
