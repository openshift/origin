package testclient

import (
	userapi "github.com/openshift/origin/pkg/user/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

func (c *FakeIdentities) List(label labels.Selector, field fields.Selector) (*userapi.IdentityList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-identities"}, &userapi.IdentityList{})
	return obj.(*userapi.IdentityList), err
}

func (c *FakeIdentities) Get(name string) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-identity", Value: name}, &userapi.Identity{})
	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Create(identity *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-identity", Value: identity}, &userapi.Identity{})
	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Update(identity *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-identity", Value: identity}, &userapi.Identity{})
	return obj.(*userapi.Identity), err
}
