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
		if err := kapi.Scheme.Convert(obj, out, nil); err != nil {
			t.Errorf("%s: did not convert: %v", k, err)
			continue
		}
		if out.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, out)
		}
	}
}

func TestDefaults(t *testing.T) {
	obj := &v1.Route{
		Spec: v1.RouteSpec{
			To:  v1.RouteTargetReference{Name: "other"},
			TLS: &v1.TLSConfig{},
		},
		Status: v1.RouteStatus{
			Ingress: []v1.RouteIngress{{}},
		},
	}
	out := &api.Route{}
	if err := kapi.Scheme.Convert(obj, out, nil); err != nil {
		t.Fatal(err)
	}
	if out.Spec.TLS.Termination != api.TLSTerminationEdge {
		t.Errorf("did not default termination: %#v", out)
	}
	if out.Spec.WildcardPolicy != api.WildcardPolicyNone {
		t.Errorf("did not default wildcard policy: %#v", out)
	}
	if out.Spec.To.Kind != "Service" {
		t.Errorf("did not default object reference kind: %#v", out)
	}
	if out.Status.Ingress[0].WildcardPolicy != api.WildcardPolicyNone {
		t.Errorf("did not default status ingress wildcard policy: %#v", out)
	}
}
