package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientRegistry struct {
	Err               error
	Clients           *api.ClientList
	Client            *api.Client
	DeletedClientName string
}

func (r *ClientRegistry) ListClients(labels labels.Selector) (*api.ClientList, error) {
	return r.Clients, r.Err
}

func (r *ClientRegistry) GetClient(name string) (*api.Client, error) {
	return r.Client, r.Err
}

func (r *ClientRegistry) CreateClient(client *api.Client) error {
	return r.Err
}

func (r *ClientRegistry) UpdateClient(client *api.Client) error {
	return r.Err
}

func (r *ClientRegistry) DeleteClient(name string) error {
	r.DeletedClientName = name
	return r.Err
}
