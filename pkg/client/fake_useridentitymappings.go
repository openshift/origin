package client

import (
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUserIdentityMappings implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUserIdentityMappings struct {
	Fake *Fake
}

func (c *FakeUserIdentityMappings) CreateOrUpdate(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, bool, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "createorupdate-useridentitymapping"})
	return nil, false, nil
}
