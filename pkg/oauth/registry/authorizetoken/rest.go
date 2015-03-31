package authorizetoken

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New returns a new AuthorizeToken for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.OAuthAuthorizeToken{}
}

func (*REST) NewList() runtime.Object {
	return &api.OAuthAuthorizeToken{}
}

// Get retrieves an AuthorizeToken by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	token, err := s.registry.GetAuthorizeToken(id)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// List retrieves a list of AuthorizeTokens that match label.
func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	tokens, err := s.registry.ListAuthorizeTokens(label)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// Create registers the given AuthorizeToken.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	token, ok := obj.(*api.OAuthAuthorizeToken)
	if !ok {
		return nil, fmt.Errorf("not an token: %#v", obj)
	}

	kapi.FillObjectMetaSystemFields(ctx, &token.ObjectMeta)

	if errs := validation.ValidateAuthorizeToken(token); len(errs) > 0 {
		return nil, kerrors.NewInvalid("token", token.Name, errs)
	}

	if err := s.registry.CreateAuthorizeToken(token); err != nil {
		return nil, err
	}
	return s.Get(ctx, token.Name)
}

// Delete asynchronously deletes an AuthorizeToken specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteAuthorizeToken(id)
}
