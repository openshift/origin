package client

import (
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUsers implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

func (c *FakeUsers) Get(name string) (*userapi.User, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-user", Value: name})
	return &userapi.User{}, nil
}
