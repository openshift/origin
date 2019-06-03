package session

import (
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/oauth-server/pkg/oauth/handlers"
)

// Store abstracts HTTP session storage of Values
type Store interface {
	// Get and decode the Values associated with the given request
	Get(r *http.Request) Values
	// Put encodes and writes the given Values to the response
	Put(w http.ResponseWriter, v Values) error
}

type Values map[interface{}]interface{}

func (v Values) GetString(key string) (string, bool) {
	str, _ := v[key].(string)
	return str, len(str) != 0
}

func (v Values) GetInt64(key string) (int64, bool) {
	i, _ := v[key].(int64)
	return i, i != 0
}

type SessionInvalidator interface {
	InvalidateAuthentication(w http.ResponseWriter, user user.Info) error
}

type SessionAuthenticator interface {
	authenticator.Request
	handlers.AuthenticationSuccessHandler
	SessionInvalidator
}
