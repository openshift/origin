package bearertoken

import (
	"net/http"
	"strings"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

type Authenticator struct {
	// auth is the token authenticator to use to validate the token
	auth authenticator.Token
	// removeHeader indicates whether the Authorization header should be removeHeaderd on successful auth
	removeHeader bool
}

func New(auth authenticator.Token, removeHeader bool) *Authenticator {
	return &Authenticator{auth, removeHeader}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return nil, false, nil
	}
	parts := strings.Split(auth, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, false, nil
	}

	token := parts[1]
	user, ok, err := a.auth.AuthenticateToken(token)
	if ok && a.removeHeader {
		req.Header.Del("Authorization")
	}
	return user, ok, err
}
