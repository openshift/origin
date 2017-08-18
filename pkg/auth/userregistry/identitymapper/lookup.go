package identitymapper

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuser "k8s.io/apiserver/pkg/authentication/user"

	authapi "github.com/openshift/origin/pkg/auth/api"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

var _ = authapi.UserIdentityMapper(&lookupIdentityMapper{})

// lookupIdentityMapper does not provision a new identity or user, it only allows identities already associated with users
type lookupIdentityMapper struct {
	mappings userclient.UserIdentityMappingInterface
	users    userclient.UserResourceInterface
}

// UserFor returns info about the user for whom identity info has been provided
func (p *lookupIdentityMapper) UserFor(info authapi.UserIdentityInfo) (kuser.Info, error) {
	mapping, err := p.mappings.Get(info.GetIdentityName(), metav1.GetOptions{})
	if err != nil {
		return nil, NewLookupError(info, err)
	}

	u, err := p.users.Get(mapping.User.Name, metav1.GetOptions{})
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
