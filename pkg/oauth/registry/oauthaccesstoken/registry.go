package oauthaccesstoken

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store AccessToken objects.
type Registry interface {
	// ListAccessTokens obtains a list of access tokens that match a selector.
	ListAccessTokens(ctx kapi.Context, selector labels.Selector) (*api.OAuthAccessTokenList, error)
	// GetAccessToken retrieves a specific access token.
	GetAccessToken(ctx kapi.Context, name string) (*api.OAuthAccessToken, error)
	// CreateAccessToken creates a new access token.
	CreateAccessToken(ctx kapi.Context, token *api.OAuthAccessToken) (*api.OAuthAccessToken, error)
	// DeleteAccessToken deletes an access token.
	DeleteAccessToken(ctx kapi.Context, name string) error
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.Creater
	rest.GracefulDeleter
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) ListAccessTokens(ctx kapi.Context, label labels.Selector) (*api.OAuthAccessTokenList, error) {
	obj, err := s.List(ctx, label, fields.Everything())
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAccessTokenList), nil
}

func (s *storage) GetAccessToken(ctx kapi.Context, name string) (*api.OAuthAccessToken, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAccessToken), nil
}

func (s *storage) CreateAccessToken(ctx kapi.Context, token *api.OAuthAccessToken) (*api.OAuthAccessToken, error) {
	obj, err := s.Create(ctx, token)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAccessToken), nil
}

func (s *storage) DeleteAccessToken(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
