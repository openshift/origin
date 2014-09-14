package useridentitymapping

import (
	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface for things that know how to store Identity objects.
type Registry interface {
	// GetOrCreateUserIdentityMapping creates or retrieves the mapping between an
	// identity and a user.
	GetOrCreateUserIdentityMapping(mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error)
}
