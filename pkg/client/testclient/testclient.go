package testclient

import (
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
// with data populated from provided path.
func NewFixtureClients(objs ...runtime.Object) (client.Interface, kclientset.Interface, kclient.Interface) {
	oc := NewSimpleFake(objs...)
	kc := fake.NewSimpleClientset(objs...)
	oldK := ktestclient.NewSimpleFake(objs...)
	return oc, kc, oldK
}

func NewErrorClients(err error) (client.Interface, kclientset.Interface, kclient.Interface) {
	oc := &Fake{}
	oc.PrependReactor("*", "*", func(action ktestclient.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	kc := &fake.Clientset{}
	kc.PrependReactor("*", "*", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	oldK := &ktestclient.Fake{}
	oc.PrependReactor("*", "*", func(action ktestclient.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return oc, kc, oldK
}
