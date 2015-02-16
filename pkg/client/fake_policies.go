package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakePolicies struct {
	Fake *Fake
}

func (c *FakePolicies) List(label, field labels.Selector) (*authorizationapi.PolicyList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-policies"})
	return &authorizationapi.PolicyList{}, nil
}

func (c *FakePolicies) Get(name string) (*authorizationapi.Policy, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-policy"})
	return &authorizationapi.Policy{}, nil
}

func (c *FakePolicies) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-policy", Value: name})
	return nil
}

func (c *FakePolicies) Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-policy"})
	return nil, nil
}
