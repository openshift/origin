package gitlab

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"

	"github.com/openshift/oauth-server/pkg/oauth/external"
	"github.com/openshift/oauth-server/pkg/oauth/external/openid"
)

const (
	// https://gitlab.com/help/integration/openid_connect_provider.md
	// Uses GitLab OIDC, requires GitLab 11.1.0 or higher
	// Earlier versions do not work: https://gitlab.com/gitlab-org/gitlab-ce/issues/47791#note_81269161
	gitlabAuthorizePath = "/oauth/authorize"
	gitlabTokenPath     = "/oauth/token"
	gitlabUserInfoPath  = "/oauth/userinfo"

	// https://gitlab.com/gitlab-org/gitlab-ce/blob/master/config/locales/doorkeeper.en.yml
	// Authenticate using OpenID Connect
	// The ability to authenticate using GitLab, and read-only access to the user's profile information and group memberships
	gitlabOIDCScope = "openid"

	// The ID of the user
	// See above comment about GitLab 11.1.0 and the custom IDTokenValidator below
	// Along with providerName, builds the identity object's Name field (see Identity.ProviderUserName)
	gitlabIDClaim = "sub"
	// The user's GitLab username
	// Used as the Name field of the user object (stored in Identity.Extra, see IdentityPreferredUsernameKey)
	gitlabPreferredUsernameClaim = "nickname"
	// The user's public email address
	// The value can optionally be used during manual provisioning (stored in Identity.Extra, see IdentityEmailKey)
	gitlabEmailClaim = "email"
	// The user's full name
	// Used as the FullName field of the user object (stored in Identity.Extra, see IdentityDisplayNameKey)
	gitlabDisplayNameClaim = "name"
)

func NewOIDCProvider(providerName, URL, clientID, clientSecret string, transport http.RoundTripper) (external.Provider, error) {
	// Create service URLs
	u, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("gitlab host URL %q is invalid", URL)
	}

	config := openid.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,

		AuthorizeURL: appendPath(*u, gitlabAuthorizePath),
		TokenURL:     appendPath(*u, gitlabTokenPath),
		UserInfoURL:  appendPath(*u, gitlabUserInfoPath),

		Scopes: []string{gitlabOIDCScope},

		IDClaims:                []string{gitlabIDClaim},
		PreferredUsernameClaims: []string{gitlabPreferredUsernameClaim},
		EmailClaims:             []string{gitlabEmailClaim},
		NameClaims:              []string{gitlabDisplayNameClaim},

		// make sure that gitlabIDClaim is a valid uint64, see above comment about GitLab 11.1.0
		IDTokenValidator: func(idTokenClaims map[string]interface{}) error {
			gitlabID, ok := idTokenClaims[gitlabIDClaim].(string)
			if !ok {
				return nil // this is an OIDC spec violation which is handled by the default code path
			}
			if reSHA256HexDigest.MatchString(gitlabID) {
				return fmt.Errorf("incompatible gitlab IDP, ID claim is SHA256 hex digest instead of digit, claims=%#v", idTokenClaims)
			}
			if !isValidUint64(gitlabID) {
				return fmt.Errorf("invalid gitlab IDP, ID claim is not a digit, claims=%#v", idTokenClaims)
			}
			return nil
		},
	}

	return openid.NewProvider(providerName, transport, config)
}

func appendPath(u url.URL, subpath string) string {
	u.Path = path.Join(u.Path, subpath)
	return u.String()
}

// Have 256 bits from hex digest
// In hexadecimal each digit encodes 4 bits
// Thus we need 64 digits to represent 256 bits
var reSHA256HexDigest = regexp.MustCompile(`^[[:xdigit:]]{64}$`)

func isValidUint64(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
