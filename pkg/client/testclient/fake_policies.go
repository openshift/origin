package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakePolicies implements PolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicies struct {
	Fake      *Fake
	Namespace string
}

func (c *FakePolicies) Get(name string) (*authorizationapi.Policy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("policies", c.Namespace, name), &authorizationapi.Policy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Policy), err
}

func (c *FakePolicies) List(opts kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("policies", c.Namespace, opts), &authorizationapi.PolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyList), err
}

func (c *FakePolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("policies", c.Namespace, name), &authorizationapi.Policy{})
	return err
}

func (c *FakePolicies) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("policies", c.Namespace, opts))
}
