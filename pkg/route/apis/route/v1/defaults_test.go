package v1

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/openshift/origin/pkg/route/apis/route"
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

func init() {
	SchemeBuilder.AddToScheme(scheme)
	route.SchemeBuilder.AddToScheme(scheme)
}

func TestDefaults(t *testing.T) {
	obj := &Route{
		Spec: RouteSpec{
			To:  RouteTargetReference{Name: "other"},
			TLS: &TLSConfig{},
		},
		Status: RouteStatus{
			Ingress: []RouteIngress{{}},
		},
	}

	obj2 := roundTrip(t, obj)
	out, ok := obj2.(*Route)
	if !ok {
		t.Errorf("Unexpected object: %v", obj2)
		t.FailNow()
	}

	if out.Spec.TLS.Termination != TLSTerminationEdge {
		t.Errorf("did not default termination: %#v", out)
	}
	if out.Spec.WildcardPolicy != WildcardPolicyNone {
		t.Errorf("did not default wildcard policy: %#v", out)
	}
	if out.Spec.To.Kind != "Service" {
		t.Errorf("did not default object reference kind: %#v", out)
	}
	if out.Status.Ingress[0].WildcardPolicy != WildcardPolicyNone {
		t.Errorf("did not default status ingress wildcard policy: %#v", out)
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(codecs.LegacyCodec(SchemeGroupVersion), obj)
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
