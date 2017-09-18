package user

import (
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// Initializer is responsible for initializing fields in a User API object from its associated Identity
type Initializer interface {
	// InitializeUser is responsible for initializing fields in a User API object from its associated Identity
	InitializeUser(identity *userapi.Identity, user *userapi.User) error
}
