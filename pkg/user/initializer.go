package user

import "github.com/openshift/origin/pkg/user/api"

type DefaultUserInitStrategy struct {
}

func NewDefaultUserInitStrategy() Initializer {
	return &DefaultUserInitStrategy{}
}

// InitializeUser implements Initializer
func (*DefaultUserInitStrategy) InitializeUser(identity *api.Identity, user *api.User) error {
	if identity.Extra != nil {
		if name, ok := identity.Extra["name"]; ok && len(name) > 0 {
			user.FullName = name
		}
	}
	return nil
}
