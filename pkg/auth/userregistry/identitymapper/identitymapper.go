package identitymapper

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	authapi "github.com/openshift/origin/pkg/auth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	uimap "github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type alwaysCreateUserIdentityToUserMapper struct {
	providerID           string
	userIdentityRegistry uimap.Registry
}

// NewAlwaysCreateUserIdentityToUserMapper always does a createOrUpdate for the passed identity
func NewAlwaysCreateUserIdentityToUserMapper(providerID string, userIdentityRegistry uimap.Registry) authapi.UserIdentityMapper {
	return &alwaysCreateUserIdentityToUserMapper{providerID, userIdentityRegistry}
}

// UserFor returns info about the user for whom identity info have been provided
func (p *alwaysCreateUserIdentityToUserMapper) UserFor(identityInfo authapi.UserIdentityInfo) (user.Info, error) {
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

	return &user.DefaultInfo{
		Name: authoritativeMapping.User.Name,
		UID:  string(authoritativeMapping.User.UID),
	}, nil
}
