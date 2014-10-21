package authenticator

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

type Token interface {
	AuthenticateToken(token string) (api.UserInfo, bool, error)
}

type Request interface {
	AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error)
}

type Password interface {
	AuthenticatePassword(user, password string) (api.UserInfo, bool, error)
}

type Assertion interface {
	AuthenticateAssertion(assertionType, data string) (api.UserInfo, bool, error)
}

type Client interface {
	AuthenticateClient(client api.Client) (api.UserInfo, bool, error)
}
