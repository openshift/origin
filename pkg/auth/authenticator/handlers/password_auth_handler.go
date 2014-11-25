package handlers

import (
	"net/http"

	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

type basicPasswordAuthHandler struct {
	realm string
}

// NewBasicPasswordAuthHandler returns a ChallengeAuthHandler that responds with a basic auth challenge for the supplied realm
func NewBasicPasswordAuthHandler(realm string) oauthhandlers.ChallengeAuthHandler {
	return &basicPasswordAuthHandler{realm}
}

// AuthenticationChallengeNeeded returns a header that indicates a basic auth challenge for the supplied realm
func (h *basicPasswordAuthHandler) AuthenticationChallengeNeeded(req *http.Request) (http.Header, error) {
	headers := http.Header{}
	headers.Add("WWW-Authenticate", "Basic realm=\""+h.realm+"\"")

	return headers, nil
}
