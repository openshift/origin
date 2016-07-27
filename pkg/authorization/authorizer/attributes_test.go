package authorizer

import (
	"testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestAuthorizationAttributes(t *testing.T) {
	// Wrapper to make sure additions to the Action interface get corresponding fields added in api.Action
	// If an additional function is required to satisfy this interface, the data for it should come from the contained authorizationapi.Action
	var _ Action = authorizationAttributesAdapter{}
}

type authorizationAttributesAdapter struct {
	attrs authorizationapi.Action
}

func (a authorizationAttributesAdapter) GetVerb() string {
	return a.attrs.Verb
}

func (a authorizationAttributesAdapter) GetAPIVersion() string {
	return a.attrs.Version
}

func (a authorizationAttributesAdapter) GetAPIGroup() string {
	return a.attrs.Group
}

func (a authorizationAttributesAdapter) GetResource() string {
	return a.attrs.Resource
}

func (a authorizationAttributesAdapter) GetResourceName() string {
	return a.attrs.ResourceName
}

func (a authorizationAttributesAdapter) GetRequestAttributes() interface{} {
	// Action doesn't currently support request attributes,
	// because they cannot be reliably serialized
	return nil
}

func (a authorizationAttributesAdapter) IsNonResourceURL() bool {
	// Action currently only supports resource authorization checks
	return false
}

func (a authorizationAttributesAdapter) GetURL() string {
	// Action currently only supports resource authorization checks
	return ""
}
