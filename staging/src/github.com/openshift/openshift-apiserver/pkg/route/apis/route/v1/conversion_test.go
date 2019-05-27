package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/davecgh/go-spew/spew"
	v1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	"github.com/openshift/origin/pkg/route/apis/route"
)

func TestFieldSelectorConversions(t *testing.T) {
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{Install},
		Kind:          v1.GroupVersion.WithKind("Route"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"spec.host", "spec.path", "spec.to.name"},
		FieldKeyEvaluatorFn:      route.RouteFieldSelector,
	}.Check(t)
}

func TestSupportingCamelConstants(t *testing.T) {
	scheme := runtime.NewScheme()
	Install(scheme)

	for k, v := range map[v1.TLSTerminationType]v1.TLSTerminationType{
		"Reencrypt":   v1.TLSTerminationReencrypt,
		"Edge":        v1.TLSTerminationEdge,
		"Passthrough": v1.TLSTerminationPassthrough,
	} {
		obj := &v1.Route{
			Spec: v1.RouteSpec{
				TLS: &v1.TLSConfig{Termination: k},
			},
		}
		scheme.Default(obj)
		if obj.Spec.TLS.Termination != v {
			t.Errorf("%s: did not default termination: %#v", k, spew.Sdump(obj))
		}
	}
}
