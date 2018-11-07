package session

import (
	"net/http"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	userNameKey = "user.name"
	userUIDKey  = "user.uid"

	// expKey is stored as an int64 unix time
	expKey = "exp"
)

type Authenticator struct {
	store  Store
	maxAge time.Duration
}

func NewAuthenticator(store Store, maxAge time.Duration) *Authenticator {
	return &Authenticator{
		store:  store,
		maxAge: maxAge,
	}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
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

	return &user.DefaultInfo{
		Name: name,
		UID:  uid,
	}, true, nil
}

func (a *Authenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, a.put(w, user.GetName(), user.GetUID(), time.Now().Add(a.maxAge).Unix())
}

func (a *Authenticator) InvalidateAuthentication(w http.ResponseWriter, req *http.Request) error {
	// zero out all fields
	return a.put(w, "", "", 0)
}

func (a *Authenticator) put(w http.ResponseWriter, name, uid string, expires int64) error {
	values := Values{}

	values[userNameKey] = name
	values[userUIDKey] = uid

	values[expKey] = expires

	return a.store.Put(w, values)
}
