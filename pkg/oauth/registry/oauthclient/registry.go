package oauthclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// Registry is an interface for things that know how to store OAuthClient objects.
type Registry interface {
	// ListClients obtains a list of clients that match a selector.
	ListClients(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientList, error)
	// GetClient retrieves a specific client.
	GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClient, error)
	// CreateClient creates a new client.
	CreateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error)
	// UpdateClient updates a client.
	UpdateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error)
	// DeleteClient deletes a client.
	DeleteClient(ctx apirequest.Context, name string) error
}

// Getter exposes a way to get a specific client.  This is useful for other registries to get scope limitations
// on particular clients.   This interface will make its easier to write a future cache on it
type Getter interface {
	GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClient, error)
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListClients(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClientList), nil
}

func (s *storage) GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClient, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClient), nil
}

func (s *storage) CreateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := s.Create(ctx, client, false)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClient), nil
}

func (s *storage) UpdateClient(ctx apirequest.Context, client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, _, err := s.Update(ctx, client.Name, rest.DefaultUpdatedObjectInfo(client, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClient), nil
}

func (s *storage) DeleteClient(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
