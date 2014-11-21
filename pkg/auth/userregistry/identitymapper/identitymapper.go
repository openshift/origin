package identitymapper

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
			ObjectMeta: kapi.ObjectMeta{
				Name: identityInfo.GetName(),
			},
			Provider: p.providerId, // Provider information from the provider plugin itself is not considered authoritative
			Extra:    identityInfo.GetExtra(),
		},
	}
	authoritativeMapping, _, err := p.userIdentityRegistry.CreateOrUpdateUserIdentityMapping(userIdentityMapping)

	ret := &authapi.DefaultUserInfo{
		Name:  authoritativeMapping.User.Name,
		UID:   authoritativeMapping.User.UID,
		Extra: authoritativeMapping.Identity.Extra,
	}
	return ret, err
}
