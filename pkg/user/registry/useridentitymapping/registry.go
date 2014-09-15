package useridentitymapping

import (
	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface for things that know how to store Identity objects.
type Registry interface {
	// CreateOrUpdateUserIdentityMapping creates or updates the mapping between an
	// identity and a user.
	CreateOrUpdateUserIdentityMapping(mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, bool, error)
}
