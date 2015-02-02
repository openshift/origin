package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientRegistry struct {
	Err               error
	Clients           *api.OAuthClientList
	Client            *api.OAuthClient
	DeletedClientName string
}

func (r *ClientRegistry) ListClients(labels labels.Selector) (*api.OAuthClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(name string) (*api.OAuthClient, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(client *api.OAuthClient) error {
	return r.Err
}

func (r *ClientRegistry) UpdateClient(client *api.OAuthClient) error {
	return r.Err
}

func (r *ClientRegistry) DeleteClient(name string) error {
	r.DeletedClientName = name
	return r.Err
}
