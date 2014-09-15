package clientauthorization

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store ClientAuthorization objects.
type Registry interface {
	ClientAuthorizationID(userName, clientName string) string
	ListClientAuthorizations(label, field labels.Selector) (*api.ClientAuthorizationList, error)
	GetClientAuthorization(id string) (*api.ClientAuthorization, error)
	CreateClientAuthorization(token *api.ClientAuthorization) error
	UpdateClientAuthorization(token *api.ClientAuthorization) error
	DeleteClientAuthorization(id string) error
}
