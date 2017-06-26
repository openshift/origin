package test

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type ClientRegistry struct {
	Err               error
	Clients           *oauthapi.OAuthClientList
	Client            *oauthapi.OAuthClient
	DeletedClientName string
}

func (r *ClientRegistry) ListClients(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) UpdateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) DeleteClient(ctx apirequest.Context, name string) error {
	r.DeletedClientName = name
	return r.Err
}
