package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

func (c *FakeIdentities) List(label labels.Selector, field fields.Selector) (*userapi.IdentityList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-identities"})
	return &userapi.IdentityList{}, nil
}

func (c *FakeIdentities) Get(name string) (*userapi.Identity, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-identity", Value: name})
	return &userapi.Identity{}, nil
}

func (c *FakeIdentities) Create(identity *userapi.Identity) (*userapi.Identity, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-identity", Value: identity})
	return &userapi.Identity{}, nil
}

func (c *FakeIdentities) Update(identity *userapi.Identity) (*userapi.Identity, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-identity", Value: identity})
	return &userapi.Identity{}, nil
}
