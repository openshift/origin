package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
	"k8s.io/kubernetes/pkg/auth/user"
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

// AuthenticationErrorHandler reacts to authentication errors
type AuthenticationErrorHandler interface {
	// AuthenticationError reacts to authentication errors, returns true if the response was written,
	// and returns any unhandled error (which should be the original error in most cases)
	AuthenticationError(error, http.ResponseWriter, *http.Request) (handled bool, err error)
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
