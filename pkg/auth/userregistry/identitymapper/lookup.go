package identitymapper

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kuser "k8s.io/kubernetes/pkg/auth/user"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type lookupIdentityMapper struct {
	mappings useridentitymapping.Registry
	users    user.Registry
}

// NewLookupIdentityMapper returns a mapper that will look up existing mappings for identities
func NewLookupIdentityMapper(mappings useridentitymapping.Registry, users user.Registry) authapi.UserIdentityMapper {
	return &lookupIdentityMapper{mappings, users}
}

// UserFor returns info about the user for whom identity info has been provided
func (p *lookupIdentityMapper) UserFor(info authapi.UserIdentityInfo) (kuser.Info, error) {
	ctx := kapi.NewContext()

	mapping, err := p.mappings.GetUserIdentityMapping(ctx, info.GetIdentityName())
	if err != nil {
		return nil, err
	}

	u, err := p.users.GetUser(ctx, mapping.User.Name)
	if err != nil {
		return nil, err
	}

	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: u.Groups,
	}, nil
}
