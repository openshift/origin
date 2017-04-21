package testclient

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
// with data populated from provided path.
func NewFixtureClients(objs ...runtime.Object) (client.Interface, kclientset.Interface) {
	oc := NewSimpleFake(OriginObjects(objs)...)
	kc := fake.NewSimpleClientset(UpstreamObjects(objs)...)
	return oc, kc
}

func NewErrorClients(err error) (client.Interface, kclientset.Interface) {
	oc := &Fake{}
	oc.PrependReactor("*", "*", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	kc := &fake.Clientset{}
	kc.PrependReactor("*", "*", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return oc, kc
}

// OriginObjects returns the origin types.
func OriginObjects(objs []runtime.Object) []runtime.Object {
	ret := []runtime.Object{}
	for _, obj := range objs {
		if !upstreamType(obj) {
			ret = append(ret, obj)
		}
	}
	return ret
}

// UpstreamObjects returns the non-origin types.
func UpstreamObjects(objs []runtime.Object) []runtime.Object {
	ret := []runtime.Object{}
	for _, obj := range objs {
		if upstreamType(obj) {
			ret = append(ret, obj)
		}
	}
	return ret
}

// upstreamType returns true for Kubernetes types.
func upstreamType(obj runtime.Object) bool {
	t := reflect.TypeOf(obj).Elem()
	return strings.Contains(t.PkgPath(), "k8s.io/kubernetes/")
}
