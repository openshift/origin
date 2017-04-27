package test

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AccessTokenRegistry struct {
	Err                    error
	AccessTokens           *api.OAuthAccessTokenList
	AccessToken            *api.OAuthAccessToken
	DeletedAccessTokenName string
}

func (r *AccessTokenRegistry) ListAccessTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*api.OAuthAccessTokenList, error) {
	return r.AccessTokens, r.Err
}

func (r *AccessTokenRegistry) GetAccessToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*api.OAuthAccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) CreateAccessToken(ctx apirequest.Context, token *api.OAuthAccessToken) (*api.OAuthAccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) DeleteAccessToken(ctx apirequest.Context, name string) error {
	r.DeletedAccessTokenName = name
	return r.Err
}
