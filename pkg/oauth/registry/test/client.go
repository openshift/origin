package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientRegistry struct {
	Err               error
	Clients           *api.OAuthClientList
	Client            *api.OAuthClient
	DeletedClientName string
}

func (r *ClientRegistry) ListClients(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(ctx kapi.Context, name string) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(ctx kapi.Context, client *api.OAuthClient) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) UpdateClient(ctx kapi.Context, client *api.OAuthClient) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) DeleteClient(ctx kapi.Context, name string) error {
	r.DeletedClientName = name
	return r.Err
}
