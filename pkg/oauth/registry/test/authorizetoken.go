package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AuthorizeTokenRegistry struct {
	Err                       error
	AuthorizeTokens           *api.OAuthAuthorizeTokenList
	AuthorizeToken            *api.OAuthAuthorizeToken
	DeletedAuthorizeTokenName string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthAuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(ctx kapi.Context, name string) (*api.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(ctx kapi.Context, token *api.OAuthAuthorizeToken) (*api.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(ctx kapi.Context, name string) error {
	r.DeletedAuthorizeTokenName = name
	return r.Err
}
