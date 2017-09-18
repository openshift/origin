package v1_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	obj := &routeapiv1.Route{
		Spec: routeapiv1.RouteSpec{
			To:  routeapiv1.RouteTargetReference{Name: "other"},
			TLS: &routeapiv1.TLSConfig{},
		},
		Status: routeapiv1.RouteStatus{
			Ingress: []routeapiv1.RouteIngress{{}},
		},
	}

	obj2 := roundTrip(t, obj)
	out, ok := obj2.(*routeapiv1.Route)
	if !ok {
		t.Errorf("Unexpected object: %v", obj2)
		t.FailNow()
	}

	if out.Spec.TLS.Termination != routeapiv1.TLSTerminationEdge {
		t.Errorf("did not default termination: %#v", out)
	}
	if out.Spec.WildcardPolicy != routeapiv1.WildcardPolicyNone {
		t.Errorf("did not default wildcard policy: %#v", out)
	}
	if out.Spec.To.Kind != "Service" {
		t.Errorf("did not default object reference kind: %#v", out)
	}
	if out.Status.Ingress[0].WildcardPolicy != routeapiv1.WildcardPolicyNone {
		t.Errorf("did not default status ingress wildcard policy: %#v", out)
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(routeapiv1.SchemeGroupVersion), obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = kapi.Scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}
