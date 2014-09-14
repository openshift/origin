package client

import (
	"github.com/openshift/origin/pkg/user/api"
)

// UserInterface exposes methods on user resources.
type UserInterface interface {
	GetUser(string) (*api.User, error)
}

// UserIdentityMappingInterface exposes methods on UserIdentityMapping resources.
type UserIdentityMappingInterface interface {
	GetOrCreateUserIdentityMapping(*api.UserIdentityMapping) (*api.UserIdentityMapping, error)
}

// GetUser returns information about a particular user or an error
func (c *Client) GetUser(name string) (result *api.User, err error) {
	result = &api.User{}
	err = c.Get().Path("users").Path(name).Do().Into(result)
	return
}

// GetOrCreateUserIdentityMapping attempts to get or create a binding between a user and an identity. If user information
// is provided, the server will check whether it matches the expected value. At this time the server will only allow creation
// when the identity is new - future APIs may allow clients to bind additional identities to an account.
func (c *Client) GetOrCreateUserIdentityMapping(mapping *api.UserIdentityMapping) (result *api.UserIdentityMapping, err error) {
	result = &api.UserIdentityMapping{}
	err = c.Post().Path("userIdentityMappings").Body(mapping).Do().Into(result)
	return
}
