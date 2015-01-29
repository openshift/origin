package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AccessTokenRegistry struct {
	Err                    error
	AccessTokens           *api.OAuthAccessTokenList
	AccessToken            *api.OAuthAccessToken
	DeletedAccessTokenName string
}

func (r *AccessTokenRegistry) ListAccessTokens(labels labels.Selector) (*api.OAuthAccessTokenList, error) {
	return r.AccessTokens, r.Err
}

func (r *AccessTokenRegistry) GetAccessToken(name string) (*api.OAuthAccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) CreateAccessToken(token *api.OAuthAccessToken) error {
	return r.Err
}

func (r *AccessTokenRegistry) UpdateAccessToken(token *api.OAuthAccessToken) error {
	return r.Err
}

func (r *AccessTokenRegistry) DeleteAccessToken(name string) error {
	r.DeletedAccessTokenName = name
	return r.Err
}
