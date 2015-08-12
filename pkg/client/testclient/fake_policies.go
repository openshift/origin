package testclient

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakePolicies implements PolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicies struct {
	Fake *Fake
}

func (c *FakePolicies) List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-policies"}, &authorizationapi.PolicyList{})
	return obj.(*authorizationapi.PolicyList), err
}

func (c *FakePolicies) Get(name string) (*authorizationapi.Policy, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-policy"}, &authorizationapi.Policy{})
	return obj.(*authorizationapi.Policy), err
}

func (c *FakePolicies) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-policy", Value: name})
	return nil
}

func (c *FakePolicies) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-policy"})
	return nil, nil
}
