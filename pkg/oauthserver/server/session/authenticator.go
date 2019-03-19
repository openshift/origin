package session

import (
	"net/http"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	userNameKey = "user.name"
	userUIDKey  = "user.uid"

	// expKey is stored as an int64 unix time
	expKey = "exp"
)

type sessionAuthenticator struct {
	store  Store
	maxAge time.Duration
}

func NewAuthenticator(store Store, maxAge time.Duration) SessionAuthenticator {
	return &sessionAuthenticator{
		store:  store,
		maxAge: maxAge,
	}
}

func (a *sessionAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	values := a.store.Get(req)

	expires, ok := values.GetInt64(expKey)
	if !ok {
		return nil, false, nil
	}

	if expires < time.Now().Unix() {
		return nil, false, nil
	}

	name, ok := values.GetString(userNameKey)
	if !ok {
		return nil, false, nil
	}

	uid, ok := values.GetString(userUIDKey)
	if !ok {
		return nil, false, nil
	}

	return &authenticator.Response{
		User: &user.DefaultInfo{
			Name: name,
			UID:  uid,
		},
	}, true, nil
}

func (a *sessionAuthenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, putUser(a.store, w, user, a.maxAge)
}

func (a *sessionAuthenticator) InvalidateAuthentication(w http.ResponseWriter, _ user.Info) error {
	// zero out all fields
	return putUser(a.store, w, &user.DefaultInfo{}, 0)
}
