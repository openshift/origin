package user

import (
	authapi "github.com/openshift/origin/pkg/auth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type DefaultUserInitStrategy struct {
}

func NewDefaultUserInitStrategy() Initializer {
	return &DefaultUserInitStrategy{}
}

// InitializeUser implements Initializer
func (*DefaultUserInitStrategy) InitializeUser(identity *userapi.Identity, user *userapi.User) error {
	if len(user.FullName) == 0 {
		user.FullName = identity.Extra[authapi.IdentityDisplayNameKey]
	}
	return nil
}
