package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store Client objects.
type Registry interface {
	// ListClients obtains a list of clients that match a selector.
	ListClients(selector labels.Selector) (*api.ClientList, error)
	// GetClient retrieves a specific client.
	GetClient(id string) (*api.Client, error)
	// CreateClient creates a new client.
	CreateClient(client *api.Client) error
	// UpdateClient updates an client.
	UpdateClient(client *api.Client) error
	// DeleteClient deletes an client.
	DeleteClient(id string) error
}
