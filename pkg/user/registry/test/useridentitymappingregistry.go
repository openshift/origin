package test

import (
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

type UserIdentityMappingRegistry struct {
	Err                        error
	Created                    bool
	UserIdentityMapping        *userapi.UserIdentityMapping
	CreatedUserIdentityMapping *userapi.UserIdentityMapping
}

func (r *UserIdentityMappingRegistry) GetUserIdentityMapping(name string) (*userapi.UserIdentityMapping, error) {
	return r.UserIdentityMapping, r.Err
}

func (r *UserIdentityMappingRegistry) CreateOrUpdateUserIdentityMapping(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, bool, error) {
	r.CreatedUserIdentityMapping = mapping
	return r.CreatedUserIdentityMapping, r.Created, r.Err
}
