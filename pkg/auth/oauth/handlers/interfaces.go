package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

type AuthenticationHandler interface {
	AuthenticationNeeded(w http.ResponseWriter, req *http.Request)
	AuthenticationError(err error, w http.ResponseWriter, req *http.Request)
}

// AuthenticationSucceeded is called when a user was successfully authenticated
// The user object may not be nil
type AuthenticationSucceeded interface {
	AuthenticationSucceeded(user api.UserInfo, w http.ResponseWriter, req *http.Request) error
}

// InvalidateAuthentication is called when an authentication is being invalidated (e.g. session timeout or log out)
// The user parameter may be nil if unknown
type AuthenticationInvalidator interface {
	InvalidateAuthentication(user api.UserInfo, w http.ResponseWriter, req *http.Request) error
}

type GrantChecker interface {
	HasAuthorizedClient(client api.Client, user api.UserInfo, grant *api.Grant) (bool, error)
}

type GrantHandler interface {
	GrantNeeded(grant *api.Grant, w http.ResponseWriter, req *http.Request)
	GrantError(err error, w http.ResponseWriter, req *http.Request)
}
