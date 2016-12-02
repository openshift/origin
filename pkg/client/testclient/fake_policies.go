package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakePolicies implements PolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicies struct {
	Fake      *Fake
	Namespace string
}

var policiesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "policies"}

func (c *FakePolicies) Get(name string) (*authorizationapi.Policy, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(policiesResource, c.Namespace, name), &authorizationapi.Policy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Policy), err
}

func (c *FakePolicies) List(opts kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(policiesResource, c.Namespace, opts), &authorizationapi.PolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyList), err
}

func (c *FakePolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(policiesResource, c.Namespace, name), &authorizationapi.Policy{})
	return err
}

func (c *FakePolicies) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewWatchAction(policiesResource, c.Namespace, opts))
}
