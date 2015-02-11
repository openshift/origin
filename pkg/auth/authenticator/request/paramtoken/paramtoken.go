package paramtoken

import (
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// Authenticator provides a way to authenticate tokens provided as a parameter
// This only exists to allow websocket connections to use an API token, since they cannot set an Authorize header
// For this authenticator to work, tokens will be part of the request URL, and are more likely to be logged or otherwise exposed.
// Every effort should be made to filter tokens from being logged when using this authenticator.
type Authenticator struct {
	param string
	auth  authenticator.Token
}

func New(param string, auth authenticator.Token) *Authenticator {
	return &Authenticator{param, auth}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	token := strings.TrimSpace(req.FormValue(a.param))
	if token == "" {
		return nil, false, nil
	}
	return a.auth.AuthenticateToken(token)
}
