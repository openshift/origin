/*
Package external implements an OAuth flow with an external identity provider
*/

package external

import (
	"net/http"

	"github.com/RangelReale/osincli"
	"github.com/openshift/origin/pkg/auth/api"
)

// Provider encapsulates the URLs, configuration, any custom authorize request parameters, and
// the method for transforming an access token into an identity, for an external OAuth provider.
type Provider interface {
	NewConfig() (*osincli.ClientConfig, error)
	AddCustomParameters(*osincli.AuthorizeRequest)
	GetUserInfo(*osincli.AccessData) (api.UserInfo, bool, error)
}

// State handles generating and verifying the state parameter round-tripped to an external OAuth flow.
// Examples: CSRF protection, post authentication redirection
type State interface {
	Generate(w http.ResponseWriter, req *http.Request) (string, error)
	Check(state string, w http.ResponseWriter, req *http.Request) (bool, error)
}
