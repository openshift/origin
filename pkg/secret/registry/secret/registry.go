package secret

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	api "github.com/openshift/origin/pkg/secret/api"
)

// Registry is an interface for things that know how to store Secrets.
type Registry interface {
	// ListSecrets obtains list of secrets that match a selector.
	ListSecrets(ctx kapi.Context, labels, field labels.Selector) (*api.SecretList, error)
	// GetSecret retrieves a specific secret.
	GetSecret(ctx kapi.Context, id string) (*api.Secret, error)
	// CreateSecret creates a new secret.
	CreateSecret(ctx kapi.Context, secret *api.Secret) error
	// UpdateSecret updates a secret.
	UpdateSecret(ctx kapi.Context, secret *api.Secret) error
	// DeleteSecret deletes a secret.
	DeleteSecret(ctx kapi.Context, id string) error
	// WatchSecrets watches secret.
	WatchSecrets(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}
