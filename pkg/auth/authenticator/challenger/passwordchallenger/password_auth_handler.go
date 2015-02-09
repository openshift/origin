package passwordchallenger

import (
	"net/http"

	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

type basicPasswordAuthHandler struct {
	realm string
}

// NewBasicAuthChallenger returns a AuthenticationChallenger that responds with a basic auth challenge for the supplied realm
func NewBasicAuthChallenger(realm string) oauthhandlers.AuthenticationChallenger {
	return &basicPasswordAuthHandler{realm}
}

// AuthenticationChallenge returns a header that indicates a basic auth challenge for the supplied realm
func (h *basicPasswordAuthHandler) AuthenticationChallenge(req *http.Request) (http.Header, error) {
	headers := http.Header{}
	headers.Add("WWW-Authenticate", "Basic realm=\""+h.realm+"\"")

	return headers, nil
}
