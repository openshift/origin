package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AuthorizeTokenRegistry struct {
	Err                     error
	AuthorizeTokens         *api.AuthorizeTokenList
	AuthorizeToken          *api.AuthorizeToken
	DeletedAuthorizeTokenId string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(labels labels.Selector) (*api.AuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(id string) (*api.AuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(token *api.AuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) UpdateAuthorizeToken(token *api.AuthorizeToken) error {
	return r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(id string) error {
	r.DeletedAuthorizeTokenId = id
	return r.Err
}
