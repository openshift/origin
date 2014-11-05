package registry

import (
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	"github.com/openshift/origin/pkg/oauth/scope"
)

type TokenAuthenticator struct {
	registry accesstoken.Registry
}

func NewTokenAuthenticator(registry accesstoken.Registry) *TokenAuthenticator {
	return &TokenAuthenticator{
		registry: registry,
	}
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (api.UserInfo, bool, error) {
	token, err := a.registry.GetAccessToken(value)
	if errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if token.CreationTimestamp.Time.Add(time.Duration(token.AuthorizeToken.ExpiresIn) * time.Second).Before(time.Now()) {
		return nil, false, nil
	}
	return &api.DefaultUserInfo{
		Name:  token.AuthorizeToken.UserName,
		UID:   token.AuthorizeToken.UserUID,
		Scope: scope.Join(token.AuthorizeToken.Scopes),
	}, true, nil
}
