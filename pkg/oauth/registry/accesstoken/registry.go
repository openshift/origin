package accesstoken

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store AccessToken objects.
type Registry interface {
	// ListAccessTokens obtains a list of access tokens that match a selector.
	ListAccessTokens(selector labels.Selector) (*api.OAuthAccessTokenList, error)
	// GetAccessToken retrieves a specific access token.
	GetAccessToken(name string) (*api.OAuthAccessToken, error)
	// CreateAccessToken creates a new access token.
	CreateAccessToken(token *api.OAuthAccessToken) error
	// UpdateAccessToken updates an access token.
	UpdateAccessToken(token *api.OAuthAccessToken) error
	// DeleteAccessToken deletes an access token.
	DeleteAccessToken(name string) error
}
