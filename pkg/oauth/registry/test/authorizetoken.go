package test

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AuthorizeTokenRegistry struct {
	Err                       error
	AuthorizeTokens           *api.OAuthAuthorizeTokenList
	AuthorizeToken            *api.OAuthAuthorizeToken
	DeletedAuthorizeTokenName string
}

func (r *AuthorizeTokenRegistry) ListAuthorizeTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*api.OAuthAuthorizeTokenList, error) {
	return r.AuthorizeTokens, r.Err
}

func (r *AuthorizeTokenRegistry) GetAuthorizeToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*api.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) CreateAuthorizeToken(ctx apirequest.Context, token *api.OAuthAuthorizeToken) (*api.OAuthAuthorizeToken, error) {
	return r.AuthorizeToken, r.Err
}

func (r *AuthorizeTokenRegistry) DeleteAuthorizeToken(ctx apirequest.Context, name string) error {
	r.DeletedAuthorizeTokenName = name
	return r.Err
}
