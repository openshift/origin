package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

type AuthenticationHandler interface {
	AuthenticationNeeded(w http.ResponseWriter, req *http.Request)
	AuthenticationError(err error, w http.ResponseWriter, req *http.Request)
}

type GrantChecker interface {
	HasAuthorizedClient(client api.Client, user api.UserInfo, grant *api.Grant) (bool, error)
}

type GrantHandler interface {
	GrantNeeded(grant *api.Grant, w http.ResponseWriter, req *http.Request)
	GrantError(err error, w http.ResponseWriter, req *http.Request)
}
