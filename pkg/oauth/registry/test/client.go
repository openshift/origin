package test

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientRegistry struct {
	Err               error
	Clients           *api.OAuthClientList
	Client            *api.OAuthClient
	DeletedClientName string
}

func (r *ClientRegistry) ListClients(ctx apirequest.Context, options *metainternal.ListOptions) (*api.OAuthClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(ctx apirequest.Context, client *api.OAuthClient) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) UpdateClient(ctx apirequest.Context, client *api.OAuthClient) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) DeleteClient(ctx apirequest.Context, name string) error {
	r.DeletedClientName = name
	return r.Err
}
