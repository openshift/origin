package v1

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/davecgh/go-spew/spew"
	v1 "github.com/openshift/api/route/v1"
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

func init() {
	Install(scheme)
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

	obj2 := roundTrip(t, obj)
	out, ok := obj2.(*v1.Route)
	if !ok {
		t.Errorf("Unexpected object: %v", obj2)
		t.FailNow()
	}

	if out.Spec.TLS.Termination != v1.TLSTerminationEdge {
		t.Errorf("did not default termination: %#v", spew.Sdump(out.Spec.TLS))
	}
	if out.Spec.WildcardPolicy != v1.WildcardPolicyNone {
		t.Errorf("did not default wildcard policy: %#v", out)
	}
	if out.Spec.To.Kind != "Service" {
		t.Errorf("did not default object reference kind: %#v", out)
	}
	if out.Status.Ingress[0].WildcardPolicy != v1.WildcardPolicyNone {
		t.Errorf("did not default status ingress wildcard policy: %#v", out)
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(codecs.LegacyCodec(v1.GroupVersion), obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}
