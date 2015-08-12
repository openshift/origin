package session

import (
	"errors"
	"net/http"

	"k8s.io/kubernetes/pkg/auth/user"
)

const UserNameKey = "user.name"
const UserUIDKey = "user.uid"

type Authenticator struct {
	store Store
	name  string
}

func NewAuthenticator(store Store, name string) *Authenticator {
	return &Authenticator{
		store: store,
		name:  name,
	}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	session, err := a.store.Get(req, a.name)
	if err != nil {
		return nil, false, err
	}

	nameObj, ok := session.Values()[UserNameKey]
	if !ok {
		return nil, false, nil
	}
	name, ok := nameObj.(string)
	if !ok {
		return nil, false, errors.New("user.name on session is not a string")
	}
	if name == "" {
		return nil, false, nil
	}

	uidObj, ok := session.Values()[UserUIDKey]
	if !ok {
		return nil, false, nil
	}
	uid, ok := uidObj.(string)
	if !ok {
		return nil, false, errors.New("user.uid on session is not a string")
	}
	// Tolerate empty string UIDs in the session

	return &user.DefaultInfo{
		Name: name,
		UID:  uid,
	}, true, nil
}

func (a *Authenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	session, err := a.store.Get(req, a.name)
	if err != nil {
		return false, err
	}
	values := session.Values()
	values[UserNameKey] = user.GetName()
	values[UserUIDKey] = user.GetUID()
	// TODO: should we save groups, scope, and extra in the session as well?
	return false, a.store.Save(w, req)
}

func (a *Authenticator) InvalidateAuthentication(w http.ResponseWriter, req *http.Request) error {
	session, err := a.store.Get(req, a.name)
	if err != nil {
		return err
	}
	session.Values()[UserNameKey] = ""
	session.Values()[UserUIDKey] = ""
	return a.store.Save(w, req)
}
