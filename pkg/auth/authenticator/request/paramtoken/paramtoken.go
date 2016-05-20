package paramtoken

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

// Authenticator provides a way to authenticate tokens provided as a parameter
// This only exists to allow websocket connections to use an API token, since they cannot set an Authorize header
// For this authenticator to work, tokens will be part of the request URL, and are more likely to be logged or otherwise exposed.
// Every effort should be made to filter tokens from being logged when using this authenticator.
type Authenticator struct {
	// param is the query param to use as a token
	param string
	// auth is the token authenticator to use to validate the token
	auth authenticator.Token
	// removeParam indicates whether the parameter should be stripped from the incoming request
	removeParam bool
}

func New(param string, auth authenticator.Token, removeParam bool) *Authenticator {
	return &Authenticator{param, auth, removeParam}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	// Only accept query param auth for websocket connections
	if !isWebSocketRequest(req) {
		return nil, false, nil
	}

	q := req.URL.Query()
	token := strings.TrimSpace(q.Get(a.param))
	if token == "" {
		return nil, false, nil
	}
	user, ok, err := a.auth.AuthenticateToken(token)
	if ok && a.removeParam {
		q.Del(a.param)
		req.URL.RawQuery = q.Encode()
	}
	return user, ok, err
}

var (
	// connectionUpgradeRegex matches any Connection header value that includes upgrade
	connectionUpgradeRegex = regexp.MustCompile("(^|.*,\\s*)upgrade($|\\s*,)")
)

// isWebSocketRequest returns true if the incoming request contains connection upgrade headers for WebSockets.
func isWebSocketRequest(req *http.Request) bool {
	return connectionUpgradeRegex.MatchString(strings.ToLower(req.Header.Get("Connection"))) && strings.ToLower(req.Header.Get("Upgrade")) == "websocket"
}
