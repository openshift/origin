package identitymapper

import (
	userapi "github.com/openshift/api/user/v1"
)

type UserToGroupMapper interface {
	GroupsFor(username string) ([]*userapi.Group, error)
}

type NoopGroupMapper struct{}

func (n NoopGroupMapper) GroupsFor(username string) ([]*userapi.Group, error) {
	return []*userapi.Group{}, nil
}

// Initializer is responsible for initializing fields in a User API object from its associated Identity
type Initializer interface {
	// InitializeUser is responsible for initializing fields in a User API object from its associated Identity
	InitializeUser(identity *userapi.Identity, user *userapi.User) error
}
