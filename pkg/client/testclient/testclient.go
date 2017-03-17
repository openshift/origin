package testclient

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
// with data populated from provided path.
func NewFixtureClients(objs ...runtime.Object) (client.Interface, kclientset.Interface) {
	oc := NewSimpleFake(oapi.OriginObjects(objs)...)
	kc := fake.NewSimpleClientset(oapi.UpstreamObjects(objs)...)
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
