package test

import (
	"github.com/openshift/origin/pkg/user/api"
)

type UserIdentityMappingRegistry struct {
	Err                        error
	Created                    bool
	CreatedUserIdentityMapping *api.UserIdentityMapping
}

func (r *UserIdentityMappingRegistry) CreateOrUpdateUserIdentityMapping(mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, bool, error) {
	r.CreatedUserIdentityMapping = mapping
	return r.CreatedUserIdentityMapping, r.Created, r.Err
}
