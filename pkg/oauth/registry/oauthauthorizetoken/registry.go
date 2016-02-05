package oauthauthorizetoken

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store AuthorizeToken objects.
type Registry interface {
	// ListAuthorizeTokens obtains a list of authorize tokens that match a selector.
	ListAuthorizeTokens(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthAuthorizeTokenList, error)
	// GetAuthorizeToken retrieves a specific authorize token.
	GetAuthorizeToken(ctx kapi.Context, name string) (*api.OAuthAuthorizeToken, error)
	// CreateAuthorizeToken creates a new authorize token.
	CreateAuthorizeToken(ctx kapi.Context, token *api.OAuthAuthorizeToken) (*api.OAuthAuthorizeToken, error)
	// DeleteAuthorizeToken deletes an authorize token.
	DeleteAuthorizeToken(ctx kapi.Context, name string) error
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

func (s *storage) ListAuthorizeTokens(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthAuthorizeTokenList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAuthorizeTokenList), nil
}

func (s *storage) GetAuthorizeToken(ctx kapi.Context, name string) (*api.OAuthAuthorizeToken, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAuthorizeToken), nil
}

func (s *storage) CreateAuthorizeToken(ctx kapi.Context, token *api.OAuthAuthorizeToken) (*api.OAuthAuthorizeToken, error) {
	obj, err := s.Create(ctx, token)
	if err != nil {
		return nil, err
	}
	return obj.(*api.OAuthAuthorizeToken), nil
}

func (s *storage) DeleteAuthorizeToken(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
