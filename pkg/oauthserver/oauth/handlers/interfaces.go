package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/oauthserver/api"
	"k8s.io/apiserver/pkg/authentication/user"
)

// AuthenticationHandler reacts to unauthenticated requests
type AuthenticationHandler interface {
	// AuthenticationNeeded reacts to unauthenticated requests, and returns true if the response was written,
	AuthenticationNeeded(client api.Client, w http.ResponseWriter, req *http.Request) (handled bool, err error)
}

// AuthenticationChallenger reacts to unauthenticated requests with challenges
type AuthenticationChallenger interface {
	// AuthenticationChallenge take a request and return whatever challenge headers are appropriate.  If none are appropriate, it should return an empty map, not nil.
	AuthenticationChallenge(req *http.Request) (header http.Header, err error)
}

// AuthenticationRedirector reacts to unauthenticated requests with redirects
type AuthenticationRedirector interface {
	// AuthenticationRedirect is expected to write a redirect to the ResponseWriter or to return an error.
	AuthenticationRedirect(w http.ResponseWriter, req *http.Request) (err error)
}

// AuthenticationRedirectors is a collection that provides a list of
// acceptable authentication mechanisms
type AuthenticationRedirectors struct {
	names         []string
	redirectorMap map[string]AuthenticationRedirector
}

// Add a name and a matching redirection routine to the list
// The entries added here will be retained in FIFO ordering.
func (ar *AuthenticationRedirectors) Add(name string, redirector AuthenticationRedirector) {
	// Initialize the map the first time
	if ar.redirectorMap == nil {
		ar.redirectorMap = make(map[string]AuthenticationRedirector, 1)
	}

	// If this name already exists in the map, ignore it.
	// This should be impossible, as uniqueness is tested by the config
	// validator, but it's always best to catch mistakes.
	if _, exists := ar.redirectorMap[name]; exists {
		return
	}

	ar.names = append(ar.names, name)
	ar.redirectorMap[name] = redirector
}

// Get the AuthenticationRedirector associated with a name.
// Also returns a boolean indicating whether the name matched
func (ar *AuthenticationRedirectors) Get(name string) (AuthenticationRedirector, bool) {
	val, exists := ar.redirectorMap[name]
	return val, exists
}

// Get a count of the AuthenticationRedirectors
func (ar *AuthenticationRedirectors) Count() int {
	return len(ar.names)
}

// Get the ordered list of names
func (ar *AuthenticationRedirectors) GetNames() []string {
	return ar.names
}

// AuthenticationErrorHandler reacts to authentication errors
type AuthenticationErrorHandler interface {
	// AuthenticationError reacts to authentication errors, returns true if the response was written,
	// and returns any unhandled error (which should be the original error in most cases)
	AuthenticationError(error, http.ResponseWriter, *http.Request) (handled bool, err error)
}

// ProviderInfo represents display information for an oauth identity provider.  This is used by the
// selection provider template to render links to login using different identity providers.
type ProviderInfo struct {
	// Name is unique and corresponds to the name of the identity provider in the oauth configuration
	Name string
	// URL to login using this identity provider
	URL string
}

// AuthenticationSelectionHandler is responsible for selecting which identity provider to use for login
type AuthenticationSelectionHandler interface {
	// SelectAuthentication will choose which identity provider to use for login or handle the request
	// If the request is being handled, such as rendering a login provider selection page, then handled will
	// be true and selected will be nil.  If the request is not handled then a provider may be selected,
	// if a provider could not be selected then selected will be nil.
	SelectAuthentication([]ProviderInfo, http.ResponseWriter, *http.Request) (selected *ProviderInfo, handled bool, err error)
}

// AuthenticationSuccessHandler reacts to a user authenticating
type AuthenticationSuccessHandler interface {
	// AuthenticationSucceeded reacts to a user authenticating, returns true if the response was written,
	// and returns false if the response was not written.
	AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error)
}

// GrantChecker is responsible for determining if a user has authorized a client for a requested grant
type GrantChecker interface {
	// HasAuthorizedClient returns true if the user has authorized the client for the requested grant
	HasAuthorizedClient(user user.Info, grant *api.Grant) (bool, error)
}

// GrantHandler handles errors during the grant process, or the client requests an unauthorized grant
type GrantHandler interface {
	// GrantNeeded reacts when a client requests an unauthorized grant, and returns true if the response was written
	// granted is true if authorization was granted
	// handled is true if the response was already written
	GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (granted, handled bool, err error)
}

// GrantErrorHandler reacts to grant errors
type GrantErrorHandler interface {
	// GrantError reacts to grant errors, returns true if the response was written,
	// and returns any unhandled error (which could be the original error)
	GrantError(error, http.ResponseWriter, *http.Request) (handled bool, err error)
}

// AuthenticationSuccessHandlers combines multiple AuthenticationSuccessHandler objects into a chain.
// On success, each handler is called. If any handler writes the response or returns an error,
// the chain is aborted.
type AuthenticationSuccessHandlers []AuthenticationSuccessHandler

func (all AuthenticationSuccessHandlers) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	for _, h := range all {
		if handled, err := h.AuthenticationSucceeded(user, state, w, req); handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}

// AuthenticationErrorHandlers combines multiple AuthenticationErrorHandler objects into a chain.
// Each handler is called in turn. If any handler writes the response, the chain is aborted.
// Otherwise, the next handler is called with the error returned from the previous handler.
type AuthenticationErrorHandlers []AuthenticationErrorHandler

func (all AuthenticationErrorHandlers) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	handled := false
	for _, h := range all {
		// Each handler gets a chance to handle or transform the error
		if handled, err = h.AuthenticationError(err, w, req); handled {
			return handled, err
		}
	}
	return handled, err
}
