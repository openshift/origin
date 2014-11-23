package identitymapper

import (
	"errors"

	authapi "github.com/openshift/origin/pkg/auth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type alwaysCreateUserIdentityToUserMapper struct {
	providerId           string
	userIdentityRegistry useridentitymapping.Registry
}

// NewAlwaysCreateProvisioner always does a createOrUpdate for the passed identity while forcing the identity.Provider to the providerId supplied here
func NewAlwaysCreateUserIdentityToUserMapper(providerId string, userIdentityRegistry useridentitymapping.Registry) authapi.UserIdentityMapper {
	return &alwaysCreateUserIdentityToUserMapper{providerId, userIdentityRegistry}
}

// ProvisionUser implements UserIdentityMapper.UserFor
func (p *alwaysCreateUserIdentityToUserMapper) UserFor(identityInfo authapi.UserIdentityInfo) (authapi.UserInfo, error) {
	userIdentityMapping := &userapi.UserIdentityMapping{
		Identity: userapi.Identity{
			Provider: p.providerId, // Provider id is imposed
			UserName: identityInfo.GetUserName(),
			Extra:    identityInfo.GetExtra(),
		},
	}
	authoritativeMapping, ok, err := p.userIdentityRegistry.CreateOrUpdateUserIdentityMapping(userIdentityMapping)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Could not map identity to user")
	}

	ret := &authapi.DefaultUserInfo{
		Name:  authoritativeMapping.User.Name,
		UID:   authoritativeMapping.User.UID,
		Extra: authoritativeMapping.Identity.Extra,
	}
	return ret, err
}
