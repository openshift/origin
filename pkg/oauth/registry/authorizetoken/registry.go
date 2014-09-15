package authorizetoken

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

// Registry is an interface for things that know how to store AuthorizeToken objects.
type Registry interface {
	// ListAuthorizeTokens obtains a list of authorize tokens that match a selector.
	ListAuthorizeTokens(selector labels.Selector) (*api.AuthorizeTokenList, error)
	// GetAuthorizeToken retrieves a specific authorize token.
	GetAuthorizeToken(name string) (*api.AuthorizeToken, error)
	// CreateAuthorizeToken creates a new authorize token.
	CreateAuthorizeToken(token *api.AuthorizeToken) error
	// UpdateAuthorizeToken updates an authorize token.
	UpdateAuthorizeToken(token *api.AuthorizeToken) error
	// DeleteAuthorizeToken deletes an authorize token.
	DeleteAuthorizeToken(name string) error
}
