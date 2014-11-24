package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AuthorizeTokenRegistry struct {
	Err                       error
	AuthorizeTokens           *api.AuthorizeTokenList
	AuthorizeToken            *api.AuthorizeToken
	DeletedAuthorizeTokenName string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(labels labels.Selector) (*api.AuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(name string) (*api.AuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(token *api.AuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) UpdateAuthorizeToken(token *api.AuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(name string) error {
	r.DeletedAuthorizeTokenName = name
	return r.Err
}
