package test

import (
	"github.com/openshift/origin/pkg/user/api"
)

type UserRegistry struct {
	Err           error
	User          *api.User
	DeletedUserID string
}

func (r *UserRegistry) GetUser(id string) (*api.User, error) {
	return r.User, r.Err
}
