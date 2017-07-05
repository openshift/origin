package oauthclientauthorization

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// Registry is an interface for things that know how to store OAuthClientAuthorization objects.
type Registry interface {
	// ClientAuthorizationName returns the name of the OAuthClientAuthorization for the given user name and client name
	ClientAuthorizationName(userName, clientName string) string
	// ListClientAuthorizations obtains a list of client auths that match a selector.
	ListClientAuthorizations(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error)
	// GetClientAuthorization retrieves a specific client auth.
	GetClientAuthorization(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error)
	// CreateClientAuthorization creates a new client auth.
	CreateClientAuthorization(ctx apirequest.Context, client *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error)
	// UpdateClientAuthorization updates a client auth.
	UpdateClientAuthorization(ctx apirequest.Context, client *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error)
	// DeleteClientAuthorization deletes a client auth.
	DeleteClientAuthorization(ctx apirequest.Context, name string) error
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

func (s *storage) ListClientAuthorizations(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClientAuthorizationList), nil
}

func (s *storage) GetClientAuthorization(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClientAuthorization), nil
}

func (s *storage) CreateClientAuthorization(ctx apirequest.Context, client *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := s.Create(ctx, client, false)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClientAuthorization), nil
}

func (s *storage) UpdateClientAuthorization(ctx apirequest.Context, client *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, _, err := s.Update(ctx, client.Name, rest.DefaultUpdatedObjectInfo(client, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthClientAuthorization), nil
}

func (s *storage) DeleteClientAuthorization(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
