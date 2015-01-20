package client

import (
	userapi "github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UsersInterface has methods to work with User resources in a namespace
type UsersInterface interface {
	Users() UserInterface
}

// UserInterface exposes methods on user resources.
type UserInterface interface {
	Get(name string) (*userapi.User, error)
}

// users implements UserIdentityMappingsNamespacer interface
type users struct {
	r *Client
}

// newUsers returns a users
func newUsers(c *Client) *users {
	return &users{
		r: c,
	}
}

// Get returns information about a particular user or an error
func (c *users) Get(name string) (result *userapi.User, err error) {
	result = &userapi.User{}
	err = c.r.Get().Resource("users").Name(name).Do().Into(result)
	return
}
