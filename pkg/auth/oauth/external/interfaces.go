// Package external implements an OAuth flow with an external identity provider
package external

import (
	"net/http"

	"github.com/RangelReale/osincli"
	authapi "github.com/openshift/origin/pkg/auth/api"
)

// Provider encapsulates the URLs, configuration, any custom authorize request parameters, and
// the method for transforming an access token into an identity, for an external OAuth provider.
type Provider interface {
	// NewConfig returns a client information that allows a standard oauth client to communicate with external oauth
	NewConfig() (*osincli.ClientConfig, error)
	// AddCustomParameters allows an external oauth provider to provide parameters that are extension to the spec.  Some providers require this.
	AddCustomParameters(*osincli.AuthorizeRequest)
	// GetUserIdentity takes the external oauth token information this and returns the user identity, isAuthenticated, and error
	GetUserIdentity(*osincli.AccessData) (authapi.UserIdentityInfo, bool, error)
}

// State handles generating and verifying the state parameter round-tripped to an external OAuth flow.
// Examples: CSRF protection, post authentication redirection
type State interface {
	Generate(w http.ResponseWriter, req *http.Request) (string, error)
	Check(state string, w http.ResponseWriter, req *http.Request) (bool, error)
}
