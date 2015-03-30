package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUsers implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

func (c *FakeUsers) List(label labels.Selector, field fields.Selector) (*userapi.UserList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-users"})
	return &userapi.UserList{}, nil
}

func (c *FakeUsers) Get(name string) (*userapi.User, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-user", Value: name})
	return &userapi.User{}, nil
}

func (c *FakeUsers) Create(user *userapi.User) (*userapi.User, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-user", Value: user})
	return &userapi.User{}, nil
}

func (c *FakeUsers) Update(user *userapi.User) (*userapi.User, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-user", Value: user})
	return &userapi.User{}, nil
}
