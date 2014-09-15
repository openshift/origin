package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientRegistry struct {
	Err             error
	Clients         *api.ClientList
	Client          *api.Client
	DeletedClientId string
}

func (r *ClientRegistry) ListClients(labels labels.Selector) (*api.ClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(id string) (*api.Client, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(client *api.Client) error {
	return r.Err
}

func (r *ClientRegistry) UpdateClient(client *api.Client) error {
	return r.Err
}

func (r *ClientRegistry) DeleteClient(id string) error {
	r.DeletedClientId = id
	return r.Err
}
