package clientauthorization

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store ClientAuthorization objects.
type Registry interface {
	ClientAuthorizationName(userName, clientName string) string
	ListClientAuthorizations(label, field labels.Selector) (*api.OAuthClientAuthorizationList, error)
	GetClientAuthorization(name string) (*api.OAuthClientAuthorization, error)
	CreateClientAuthorization(token *api.OAuthClientAuthorization) error
	UpdateClientAuthorization(token *api.OAuthClientAuthorization) error
	DeleteClientAuthorization(name string) error
}
