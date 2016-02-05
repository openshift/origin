package oauthclientauthorization

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store OAuthClientAuthorization objects.
type Registry interface {
	// ClientAuthorizationName returns the name of the OAuthClientAuthorization for the given user name and client name
	ClientAuthorizationName(userName, clientName string) string
	// ListClientAuthorizations obtains a list of client auths that match a selector.
	ListClientAuthorizations(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthClientAuthorizationList, error)
	// GetClientAuthorization retrieves a specific client auth.
	GetClientAuthorization(ctx kapi.Context, name string) (*api.OAuthClientAuthorization, error)
	// CreateClientAuthorization creates a new client auth.
	CreateClientAuthorization(ctx kapi.Context, client *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error)
	// UpdateClientAuthorization updates a client auth.
	UpdateClientAuthorization(ctx kapi.Context, client *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error)
	// DeleteClientAuthorization deletes a client auth.
	DeleteClientAuthorization(ctx kapi.Context, name string) error
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

func (s *storage) ClientAuthorizationName(userName, clientName string) string {
	return userName + ":" + clientName
}

func (s *storage) ListClientAuthorizations(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthClientAuthorizationList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthClientAuthorizationList), nil
}

func (s *storage) GetClientAuthorization(ctx kapi.Context, name string) (*api.OAuthClientAuthorization, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthClientAuthorization), nil
}

func (s *storage) CreateClientAuthorization(ctx kapi.Context, client *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	obj, err := s.Create(ctx, client)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthClientAuthorization), nil
}

func (s *storage) UpdateClientAuthorization(ctx kapi.Context, client *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	obj, _, err := s.Update(ctx, client)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthClientAuthorization), nil
}

func (s *storage) DeleteClientAuthorization(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
