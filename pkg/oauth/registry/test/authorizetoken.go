package test

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type AuthorizeTokenRegistry struct {
	Err                       error
	AuthorizeTokens           *oauthapi.OAuthAuthorizeTokenList
	AuthorizeToken            *oauthapi.OAuthAuthorizeToken
	DeletedAuthorizeTokenName string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthAuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(ctx apirequest.Context, token *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(ctx apirequest.Context, name string) error {
	r.DeletedAuthorizeTokenName = name
	return r.Err
}
