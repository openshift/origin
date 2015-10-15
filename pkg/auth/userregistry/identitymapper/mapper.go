package identitymapper

import (
	"fmt"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	mappingregistry "github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type MappingMethodType string

const (
	// MappingMethodLookup does not provision a new identity or user, it only allows identities already associated with users
	MappingMethodLookup MappingMethodType = "lookup"

	// MappingMethodClaim associates a new identity with a user with the identity's preferred username
	// if no other identities are already associated with the user
	MappingMethodClaim MappingMethodType = "claim"

	// MappingMethodAdd associates a new identity with a user with the identity's preferred username,
	// creating the user if needed, and adding to any existing identities associated with the user
	MappingMethodAdd MappingMethodType = "add"

	// MappingMethodGenerate finds an available username for a new identity, based on its preferred username
	// If a user with the preferred username already exists, a unique username is generated
	MappingMethodGenerate MappingMethodType = "generate"
)

// NewIdentityUserMapper returns a UserIdentityMapper that does the following:
// 1. Returns an existing user if the identity exists and is associated with an existing user
// 2. Returns an error if the identity exists and is not associated with a user (or is associated with a missing user)
// 3. Handles new identities according to the requested method
func NewIdentityUserMapper(identities identityregistry.Registry, users userregistry.Registry, method MappingMethodType) (authapi.UserIdentityMapper, error) {
	// initUser initializes fields in a User API object from its associated Identity
	// called when adding the first Identity to a User (during create or update of a User)
	initUser := user.NewDefaultUserInitStrategy()

	switch method {
	case MappingMethodLookup:
		mappingStorage := mappingregistry.NewREST(users, identities)
		mappingRegistry := mappingregistry.NewRegistry(mappingStorage)
		return &lookupIdentityMapper{mappingRegistry, users}, nil

	case MappingMethodClaim:
		return &provisioningIdentityMapper{identities, users, NewStrategyClaim(users, initUser)}, nil

	case MappingMethodAdd:
		return &provisioningIdentityMapper{identities, users, NewStrategyAdd(users, initUser)}, nil

	case MappingMethodGenerate:
		return &provisioningIdentityMapper{identities, users, NewStrategyGenerate(users, initUser)}, nil

	default:
		return nil, fmt.Errorf("unsupported mapping method %q", method)
	}
}
