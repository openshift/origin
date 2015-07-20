package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUsers implements UsersInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

func (c *FakeUsers) List(label labels.Selector, field fields.Selector) (*userapi.UserList, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "list-users"}, &userapi.UserList{})
	return obj.(*userapi.UserList), err
}

func (c *FakeUsers) Get(name string) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "get-user", Value: name}, &userapi.User{})
	return obj.(*userapi.User), err
}

func (c *FakeUsers) Create(user *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-user", Value: user}, &userapi.User{})
	return obj.(*userapi.User), err
}

func (c *FakeUsers) Update(user *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "update-user", Value: user}, &userapi.User{})
	return obj.(*userapi.User), err
}
