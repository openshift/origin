package user

import (
	"github.com/openshift/origin/pkg/user/api"
)

type DefaultUserInitStrategy struct {
}

func NewDefaultUserInitStrategy() Initializer {
	return &DefaultUserInitStrategy{}
}

// InitializeUser implements Initializer
func (*DefaultUserInitStrategy) InitializeUser(identity *api.Identity, user *api.User) error {
	if identity.Extra == nil {
		return nil
	}
	name, ok := identity.Extra["name"]
	if !ok {
		name = identity.Name
	}
	user.FullName = name
	return nil
}
