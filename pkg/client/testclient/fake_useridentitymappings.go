package testclient

import (
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUserIdentityMappings implements UserIdentityMappingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUserIdentityMappings struct {
	Fake *Fake
}

func (c *FakeUserIdentityMappings) Get(name string) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-useridentitymapping", Value: name}, &userapi.UserIdentityMapping{})
	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Create(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-useridentitymapping", Value: mapping}, &userapi.UserIdentityMapping{})
	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Update(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-useridentitymapping", Value: mapping}, &userapi.UserIdentityMapping{})
	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-useridentitymapping", Value: name})
	return nil
}
