package allowanypassword

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/identitymapper"
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
	// Since this IDP doesn't validate usernames or passwords, disallow usernames consisting entirely of spaces
	// Normalize usernames by removing leading/trailing spaces
	username = strings.TrimSpace(username)

	if username == "" || password == "" {
		return nil, false, nil
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, username)

	// TODO remove this
	// add three random groups in a confined range
	// allows us to play around with the semantics of identity metadata (dedupe, sort, filter, map, etc)
	identity.ProviderGroups = []string{randomGroup(), randomGroup(), randomGroup()}

	return identitymapper.UserFor(a.identityMapper, identity)
}

func randomGroup() string {
	// choose a separator based on a coin flip
	sep := "-" // one that is OK
	if rand.Intn(2) == 0 {
		sep = ":" // and one that is filtered out
	}
	// randomly pick one of five possible groups with the given separator
	return fmt.Sprintf("group%s%d", sep, rand.Intn(5))
}
