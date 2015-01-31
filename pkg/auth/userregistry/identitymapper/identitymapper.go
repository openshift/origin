package identitymapper

import (
	authapi "github.com/openshift/origin/pkg/auth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type alwaysCreateUserIdentityToUserMapper struct {
	providerID           string
	userIdentityRegistry useridentitymapping.Registry
}

// NewAlwaysCreateProvisioner always does a createOrUpdate for the passed identity while forcing the identity.Provider to the providerID supplied here
func NewAlwaysCreateUserIdentityToUserMapper(providerID string, userIdentityRegistry useridentitymapping.Registry) authapi.UserIdentityMapper {
	return &alwaysCreateUserIdentityToUserMapper{providerID, userIdentityRegistry}
}

// ProvisionUser implements UserIdentityMapper.UserFor
func (p *alwaysCreateUserIdentityToUserMapper) UserFor(identityInfo authapi.UserIdentityInfo) (authapi.UserInfo, error) {
	userIdentityMapping := &userapi.UserIdentityMapping{
		Identity: userapi.Identity{
			Provider: p.providerID, // Provider id is imposed
			UserName: identityInfo.GetUserName(),
			Extra:    identityInfo.GetExtra(),
		},
	}
	authoritativeMapping, _ /*created*/, err := p.userIdentityRegistry.CreateOrUpdateUserIdentityMapping(userIdentityMapping)
	if err != nil {
		return nil, err
	}

	ret := &authapi.DefaultUserInfo{
		Name:  authoritativeMapping.User.Name,
		UID:   string(authoritativeMapping.User.UID),
		Extra: authoritativeMapping.Identity.Extra,
	}
	return ret, err
}
