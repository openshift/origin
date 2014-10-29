package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

type AuthenticationHandler interface {
	AuthenticationNeeded(w http.ResponseWriter, req *http.Request)
	AuthenticationError(err error, w http.ResponseWriter, req *http.Request)
}

type AuthenticationSuccessHandler interface {
	AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) error
}

type AuthenticationErrorHandler interface {
	AuthenticationError(err error, w http.ResponseWriter, req *http.Request)
}

type GrantChecker interface {
	HasAuthorizedClient(client api.Client, user api.UserInfo, grant *api.Grant) (bool, error)
}

type GrantHandler interface {
	GrantNeeded(client api.Client, user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request)
	GrantError(err error, w http.ResponseWriter, req *http.Request)
}

// AuthenticationSuccessHandlers combines multiple AuthenticationSuccessHandler objects into a chain.
// On success, each handler is called. If any handler returns an error, the chain is aborted.
type AuthenticationSuccessHandlers []AuthenticationSuccessHandler

func (all AuthenticationSuccessHandlers) AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) error {
	for _, h := range all {
		if err := h.AuthenticationSucceeded(user, state, w, req); err != nil {
			return err
		}
	}
	return nil
}
