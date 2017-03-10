package identitymapper

import (
	"fmt"

	kuser "k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"

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
		return nil, NewLookupError(info, err)
	}

	u, err := p.users.GetUser(ctx, mapping.User.Name)
	if err != nil {
		return nil, NewLookupError(info, err)
	}

	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: u.Groups,
	}, nil
}

type lookupError struct {
	Identity authapi.UserIdentityInfo
	CausedBy error
}

func IsLookupError(err error) bool {
	_, ok := err.(lookupError)
	return ok
}
func NewLookupError(info authapi.UserIdentityInfo, err error) error {
	return lookupError{Identity: info, CausedBy: err}
}

func (c lookupError) Error() string {
	return fmt.Sprintf("lookup of user for %q failed: %v", c.Identity.GetIdentityName(), c.CausedBy)
}
