package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AuthorizeTokenRegistry struct {
	Err                       error
	AuthorizeTokens           *api.OAuthAuthorizeTokenList
	AuthorizeToken            *api.OAuthAuthorizeToken
	DeletedAuthorizeTokenName string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(labels labels.Selector) (*api.OAuthAuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(name string) (*api.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(token *api.OAuthAuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) UpdateAuthorizeToken(token *api.OAuthAuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(name string) error {
	r.DeletedAuthorizeTokenName = name
	return r.Err
}
