package session

import (
	"errors"
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

const UserNameKey = "user.name"

type SessionAuthenticator struct {
	store Store
	name  string
}

func NewSessionAuthenticator(store Store, name string) *SessionAuthenticator {
	return &SessionAuthenticator{
		store: store,
		name:  name,
	}
}

func (a *SessionAuthenticator) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
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

	return &api.DefaultUserInfo{
		Name: name,
	}, true, nil
}

func (a *SessionAuthenticator) AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) error {
	session, err := a.store.Get(req, a.name)
	if err != nil {
		return err
	}
	values := session.Values()
	values[UserNameKey] = user.GetName()
	return a.store.Save(w, req)
}

func (a *SessionAuthenticator) InvalidateAuthentication(context api.UserInfo, w http.ResponseWriter, req *http.Request) error {
	session, err := a.store.Get(req, a.name)
	if err != nil {
		return err
	}
	session.Values()[UserNameKey] = ""
	return a.store.Save(w, req)
}
