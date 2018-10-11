package identitymapper

import (
	kuser "k8s.io/apiserver/pkg/authentication/user"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
)

type groupsMapper struct {
	delegate authapi.UserIdentityMapper
}

func (p *groupsMapper) UserFor(identityInfo authapi.UserIdentityInfo) (kuser.Info, error) {
	user, err := p.delegate.UserFor(identityInfo)
	if err != nil {
		return nil, err
	}
	// if there are no groups then we do not need to include any extra information via authapi.UserIdentityMetadata
	groups := identityInfo.GetProviderGroups()
	if len(groups) == 0 {
		return user, nil
	}
	return authapi.NewDefaultUserIdentityMetadata(user, identityInfo.GetProviderName(), groups), nil
}
