package user

import (
	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface for things that know how to store User objects.
type Registry interface {
	GetUser(name string) (*api.User, error)
}
