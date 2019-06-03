package allowanypassword

import (
	"context"
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	authapi "github.com/openshift/oauth-server/pkg/api"
	"github.com/openshift/oauth-server/pkg/authenticator/identitymapper"
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
func (a alwaysAcceptPasswordAuthenticator) AuthenticatePassword(ctx context.Context, username, password string) (*authenticator.Response, bool, error) {
	// Since this IDP doesn't validate usernames or passwords, disallow usernames consisting entirely of spaces
	// Normalize usernames by removing leading/trailing spaces
	username = strings.TrimSpace(username)

	if username == "" || password == "" {
		return nil, false, nil
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, username)

	return identitymapper.ResponseFor(a.identityMapper, identity)
}
