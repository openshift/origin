package identitymapper

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kuser "github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"

	authapi "github.com/openshift/origin/pkg/auth/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type lookupIdentityMapper struct {
	registry useridentitymapping.Registry
}

// NewLookupIdentityMapper returns a mapper that will look up existing mappings for identities
func NewLookupIdentityMapper(registry useridentitymapping.Registry) authapi.UserIdentityMapper {
	return &lookupIdentityMapper{registry}
}

// UserFor returns info about the user for whom identity info has been provided
func (p *lookupIdentityMapper) UserFor(info authapi.UserIdentityInfo) (kuser.Info, error) {
	name := identityregistry.IdentityName(info.GetProviderName(), info.GetProviderUserName())

	mapping, err := p.registry.GetUserIdentityMapping(kapi.NewContext(), name)
	if err != nil {
		return nil, err
	}

	return &kuser.DefaultInfo{
		Name: mapping.User.Name,
		UID:  string(mapping.User.UID),
	}, nil
}
