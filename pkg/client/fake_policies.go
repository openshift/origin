package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

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
