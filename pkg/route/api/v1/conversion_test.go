package v1_test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/v1"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Route",
		// Ensure all currently returned labels are supported
		api.RouteToSelectableFields(&api.Route{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"spec.host", "spec.path", "spec.to.name",
	)
}

func TestSupportingCamelConstants(t *testing.T) {
	for k, v := range map[v1.TLSTerminationType]api.TLSTerminationType{
		"Reencrypt":   api.TLSTerminationReencrypt,
		"Edge":        api.TLSTerminationEdge,
		"Passthrough": api.TLSTerminationPassthrough,
	} {
		obj := &v1.TLSConfig{Termination: k}
		out := &api.TLSConfig{}
		if err := kapi.Scheme.Convert(obj, out); err != nil {
			t.Errorf("%s: did not convert: %v", k, err)
			continue
		}
		if out.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, out)
		}
	}
}
