package request

import (
	"errors"
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
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

func (a *Authenticator) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
	obj, ok := a.context.Get(req)
	if !ok {
		return nil, false, nil
	}
	user, ok := obj.(api.UserInfo)
	if !ok {
		return nil, false, errors.New("the context object is not an api.UserInfo")
	}
	return user, true, nil
}
