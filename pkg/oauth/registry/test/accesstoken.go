package test

import (
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AccessTokenRegistry struct {
	Err                    error
	AccessTokens           *api.OAuthAccessTokenList
	AccessToken            *api.OAuthAccessToken
	DeletedAccessTokenName string
}

func (r *AccessTokenRegistry) ListAccessTokens(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthAccessTokenList, error) {
	return r.AccessTokens, r.Err
}

func (r *AccessTokenRegistry) GetAccessToken(ctx kapi.Context, name string) (*api.OAuthAccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) CreateAccessToken(ctx kapi.Context, token *api.OAuthAccessToken) (*api.OAuthAccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) DeleteAccessToken(ctx kapi.Context, name string) error {
	r.DeletedAccessTokenName = name
	return r.Err
}
