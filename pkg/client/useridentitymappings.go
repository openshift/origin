package client

import (
	userapi "github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UserIdentityMappingsInterface has methods to work with UserIdentityMapping resources in a namespace
type UserIdentityMappingsInterface interface {
	UserIdentityMappings() UserIdentityMappingInterface
}

// UserIdentityMappingInterface exposes methods on UserIdentityMapping resources.
type UserIdentityMappingInterface interface {
	CreateOrUpdate(*userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, bool, error)
}

// userIdentityMappings implements UserIdentityMappingsNamespacer interface
type userIdentityMappings struct {
	r *Client
}

// newUserIdentityMappings returns a userIdentityMappings
func newUserIdentityMappings(c *Client) *userIdentityMappings {
	return &userIdentityMappings{
		r: c,
	}
}

// CreateOrUpdate attempts to get or create a binding between a user and an identity. If user information
// is provided, the server will check whether it matches the expected value. At this time the server will only allow creation
// when the identity is new - future APIs may allow clients to bind additional identities to an account.
func (c *userIdentityMappings) CreateOrUpdate(mapping *userapi.UserIdentityMapping) (result *userapi.UserIdentityMapping, created bool, err error) {
	result = &userapi.UserIdentityMapping{}
	err = c.r.Put().Resource("userIdentityMappings").Name(mapping.Name).Body(mapping).Do().WasCreated(&created).Into(result)
	return
}
