package identitymapper

import (
	userapi "github.com/openshift/api/user/v1"
	authapi "github.com/openshift/origin/pkg/oauthserver/api"
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
