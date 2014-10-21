package test

import (
	"github.com/openshift/origin/pkg/user/api"
)

type UserRegistry struct {
	Err           error
	Users         *api.UserList
	User          *api.User
	Mapping       *api.UserIdentityMapping
	DeletedUserId string
}

func (r *UserRegistry) GetUser(id string) (*api.User, error) {
	return r.User, r.Err
}

func (r *UserRegistry) GetOrCreateUserIdentityMapping(mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error) {
	return r.Mapping, r.Err
}
