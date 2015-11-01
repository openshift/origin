package allowanypassword

import (
	"fmt"

	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

// alwaysAcceptPasswordAuthenticator approves any login attempt with non-blank username and password
type alwaysAcceptPasswordAuthenticator struct {
	providerName   string
	identityMapper authapi.UserIdentityMapper
}

// New creates a new password authenticator that approves any login attempt with non-blank username and password
func New(providerName string, identityMapper authapi.UserIdentityMapper) authenticator.Password {
	return &alwaysAcceptPasswordAuthenticator{providerName, identityMapper}
}

// AuthenticatePassword approves any login attempt with non-blank username and password
func (a alwaysAcceptPasswordAuthenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	if username == "" || password == "" {
		return nil, false, nil
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, username)
	user, err := a.identityMapper.UserFor(identity)
	if err != nil {
		return nil, false, fmt.Errorf("error creating or updating mapping for: %#v due to %v", identity, err)
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}
