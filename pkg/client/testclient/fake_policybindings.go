package testclient

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakePolicyBindings implements PolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicyBindings struct {
	Fake *Fake
}

func (c *FakePolicyBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-policyBindings"}, &authorizationapi.PolicyBindingList{})
	return obj.(*authorizationapi.PolicyBindingList), err
}

func (c *FakePolicyBindings) Get(name string) (*authorizationapi.PolicyBinding, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-policyBinding"}, &authorizationapi.PolicyBinding{})
	return obj.(*authorizationapi.PolicyBinding), err
}

func (c *FakePolicyBindings) Create(policyBinding *authorizationapi.PolicyBinding) (*authorizationapi.PolicyBinding, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-policyBinding", Value: policyBinding}, &authorizationapi.PolicyBinding{})
	return obj.(*authorizationapi.PolicyBinding), err
}

func (c *FakePolicyBindings) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-policyBinding", Value: name})
	return nil
}

func (c *FakePolicyBindings) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-policyBinding"})
	return nil, nil
}
