package registry

import (
	"errors"
	"time"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	"github.com/openshift/origin/pkg/oauth/scope"
)

type TokenAuthenticator struct {
	registry accesstoken.Registry
}

var ErrExpired = errors.New("Token is expired")

func NewTokenAuthenticator(registry accesstoken.Registry) *TokenAuthenticator {
	return &TokenAuthenticator{
		registry: registry,
	}
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (api.UserInfo, bool, error) {
	token, err := a.registry.GetAccessToken(value)
	if err != nil {
		return nil, false, err
	}
	if token.CreationTimestamp.Time.Add(time.Duration(token.ExpiresIn) * time.Second).Before(time.Now()) {
		return nil, false, ErrExpired
	}
	return &api.DefaultUserInfo{
		Name:  token.UserName,
		UID:   token.UserUID,
		Scope: scope.Join(token.Scopes),
	}, true, nil
}
