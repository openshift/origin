package client

import (
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUserIdentityMappings implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUserIdentityMappings struct {
	Fake *Fake
}

func (c *FakeUserIdentityMappings) Get(name string) (*userapi.UserIdentityMapping, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-useridentitymapping", Value: name})
	return &userapi.UserIdentityMapping{}, nil
}

func (c *FakeUserIdentityMappings) Create(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-useridentitymapping", Value: mapping})
	return &userapi.UserIdentityMapping{}, nil
}

func (c *FakeUserIdentityMappings) Update(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-useridentitymapping", Value: mapping})
	return &userapi.UserIdentityMapping{}, nil
}

func (c *FakeUserIdentityMappings) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-useridentitymapping", Value: name})
	return nil
}
