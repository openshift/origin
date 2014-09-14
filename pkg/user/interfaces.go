package user

import (
	"github.com/openshift/origin/pkg/user/api"
)

type Initializer interface {
	InitializeUser(identity *api.Identity, user *api.User) error
}
