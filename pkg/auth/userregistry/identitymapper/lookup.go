package identitymapper

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kuser "k8s.io/kubernetes/pkg/auth/user"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

var _ = authapi.UserIdentityMapper(&lookupIdentityMapper{})

// lookupIdentityMapper does not provision a new identity or user, it only allows identities already associated with users
type lookupIdentityMapper struct {
	mappings useridentitymapping.Registry
	users    user.Registry
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
