package identitymapper

import (
	"fmt"

	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	mappingregistry "github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

// registeredMappers contains functions to build UserIdentityMappers by MappingMethodType.
var registeredMappers = map[MappingMethodType]instantiateMapper{}

// instantiateMapper knows how to make a UserIdentityMapper.
type instantiateMapper func(identities identityregistry.Registry, users userregistry.Registry, initializer user.Initializer, mappingMethodConfig *runtime.EmbeddedObject) authapi.UserIdentityMapper

// registerMapper registers a builder for a UserIdentityMapper for the given method.
func registerMapper(method MappingMethodType, fn instantiateMapper) {
	registeredMappers[method] = fn
}

// RegisteredMappingMethodTypes returns a set of all known MappingMethodTypes as strings.
func RegisteredMappingMethodTypes() sets.String {
	keys := make([]string, 0, len(registeredMappers))
	for k := range registeredMappers {
		keys = append(keys, string(k))
	}
	return sets.NewString(keys...)
}

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

func init() {
	registerMapper(MappingMethodLookup, func(identities identityregistry.Registry, users userregistry.Registry, initializer user.Initializer, mappingMethodConfig *runtime.EmbeddedObject) authapi.UserIdentityMapper {
		mappingStorage := mappingregistry.NewREST(users, identities)
		mappingRegistry := mappingregistry.NewRegistry(mappingStorage)
		return &lookupIdentityMapper{mappingRegistry, users}
	})
	registerMapper(MappingMethodAdd, func(identities identityregistry.Registry, users userregistry.Registry, initializer user.Initializer, mappingMethodConfig *runtime.EmbeddedObject) authapi.UserIdentityMapper {
		return &provisioningIdentityMapper{identities, users, NewStrategyAdd(users, user.NewDefaultUserInitStrategy())}
	})
	registerMapper(MappingMethodClaim, func(identities identityregistry.Registry, users userregistry.Registry, initializer user.Initializer, mappingMethodConfig *runtime.EmbeddedObject) authapi.UserIdentityMapper {
		return &provisioningIdentityMapper{identities, users, NewStrategyClaim(users, user.NewDefaultUserInitStrategy())}
	})
	registerMapper(MappingMethodGenerate, func(identities identityregistry.Registry, users userregistry.Registry, initializer user.Initializer, mappingMethodConfig *runtime.EmbeddedObject) authapi.UserIdentityMapper {
		return &provisioningIdentityMapper{identities, users, NewStrategyGenerate(users, user.NewDefaultUserInitStrategy())}
	})
}

// NewIdentityUserMapper returns a UserIdentityMapper that does the following:
// 1. Returns an existing user if the identity exists and is associated with an existing user
// 2. Returns an error if the identity exists and is not associated with a user (or is associated with a missing user)
// 3. Handles new identities according to the requested method
func NewIdentityUserMapper(identities identityregistry.Registry, users userregistry.Registry, method MappingMethodType, mappingMethodConfig *runtime.EmbeddedObject) (authapi.UserIdentityMapper, error) {
	// initUser initializes fields in a User API object from its associated Identity
	// called when adding the first Identity to a User (during create or update of a User)
	initUser := user.NewDefaultUserInitStrategy()

	instantiateMapper, ok := registeredMappers[method]
	if !ok {
		return nil, fmt.Errorf("unsupported mapping method %q", method)
	}
	return instantiateMapper(identities, users, initUser, mappingMethodConfig), nil
}
