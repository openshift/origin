package oauthaccesstoken

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// Registry is an interface for things that know how to store AccessToken objects.
type Registry interface {
	// ListAccessTokens obtains a list of access tokens that match a selector.
	ListAccessTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthAccessTokenList, error)
	// GetAccessToken retrieves a specific access token.
	GetAccessToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthAccessToken, error)
	// CreateAccessToken creates a new access token.
	CreateAccessToken(ctx apirequest.Context, token *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error)
	// DeleteAccessToken deletes an access token.
	DeleteAccessToken(ctx apirequest.Context, name string) error
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

func (s *storage) ListAccessTokens(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthAccessTokenList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAccessTokenList), nil
}

func (s *storage) GetAccessToken(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthAccessToken, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAccessToken), nil
}

func (s *storage) CreateAccessToken(ctx apirequest.Context, token *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error) {
	obj, err := s.Create(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return obj.(*oauthapi.OAuthAccessToken), nil
}

func (s *storage) DeleteAccessToken(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
