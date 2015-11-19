package user

import (
	"github.com/openshift/origin/pkg/user/api"
)

// Initializer is responsible for initializing fields in a User API object from its associated Identity
type Initializer interface {
	// InitializeUser is responsible for initializing fields in a User API object from its associated Identity
	InitializeUser(identity *api.Identity, user *api.User) error
}
