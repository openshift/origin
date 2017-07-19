package oauthauthorizetoken

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// Registry is an interface for things that know how to store AuthorizeToken objects.
type Registry interface {
	// ListAuthorizeTokens obtains a list of authorize tokens that match a selector.
	ListAuthorizeTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthAuthorizeTokenList, error)
	// GetAuthorizeToken retrieves a specific authorize token.
	GetAuthorizeToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthAuthorizeToken, error)
	// CreateAuthorizeToken creates a new authorize token.
	CreateAuthorizeToken(ctx apirequest.Context, token *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error)
	// DeleteAuthorizeToken deletes an authorize token.
	DeleteAuthorizeToken(ctx apirequest.Context, name string) error
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

func (s *storage) ListAuthorizeTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthAuthorizeTokenList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAuthorizeTokenList), nil
}

func (s *storage) GetAuthorizeToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAuthorizeToken), nil
}

func (s *storage) CreateAuthorizeToken(ctx apirequest.Context, token *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := s.Create(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAuthorizeToken), nil
}

func (s *storage) DeleteAuthorizeToken(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
