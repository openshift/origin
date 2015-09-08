package contextrequest

import (
	"errors"
	"net/http"

	"k8s.io/kubernetes/pkg/auth/user"
)

type Context interface {
	Get(req *http.Request) (interface{}, bool)
}

type Authenticator struct {
	context Context
}

func NewAuthenticator(context Context) *Authenticator {
	return &Authenticator{context}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	obj, ok := a.context.Get(req)
	if !ok {
		return nil, false, nil
	}
	user, ok := obj.(user.Info)
	if !ok {
		return nil, false, errors.New("the context object is not a user.Info")
	}
	return user, true, nil
}
