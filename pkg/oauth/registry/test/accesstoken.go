package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type AccessTokenRegistry struct {
	Err                    error
	AccessTokens           *api.AccessTokenList
	AccessToken            *api.AccessToken
	DeletedAccessTokenName string
}

func (r *AccessTokenRegistry) ListAccessTokens(labels labels.Selector) (*api.AccessTokenList, error) {
	return r.AccessTokens, r.Err
}

func (r *AccessTokenRegistry) GetAccessToken(name string) (*api.AccessToken, error) {
	return r.AccessToken, r.Err
}

func (r *AccessTokenRegistry) CreateAccessToken(token *api.AccessToken) error {
	return r.Err
}

func (r *AccessTokenRegistry) UpdateAccessToken(token *api.AccessToken) error {
	return r.Err
}

func (r *AccessTokenRegistry) DeleteAccessToken(name string) error {
	r.DeletedAccessTokenName = name
	return r.Err
}
