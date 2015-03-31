package clientauthorization

import (
	"errors"
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

// New returns a new ClientAuthorization for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.OAuthClientAuthorization{}
}

func (*REST) NewList() runtime.Object {
	return &api.OAuthClientAuthorization{}
}

// Get retrieves an ClientAuthorization by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	authorization, err := s.registry.GetClientAuthorization(id)
	if err != nil {
		return nil, err
	}
	return authorization, nil
}

// List retrieves a list of ClientAuthorizations that match selector.
func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return s.registry.ListClientAuthorizations(label, field)
}

// Create registers the given ClientAuthorization.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	authorization, ok := obj.(*api.OAuthClientAuthorization)
	if !ok {
		return nil, fmt.Errorf("not an authorization: %#v", obj)
	}

	if authorization.UserName == "" || authorization.ClientName == "" {
		return nil, errors.New("invalid authorization")
	}

	authorization.Name = s.registry.ClientAuthorizationName(authorization.UserName, authorization.ClientName)
	kapi.FillObjectMetaSystemFields(ctx, &authorization.ObjectMeta)

	if errs := validation.ValidateClientAuthorization(authorization); len(errs) > 0 {
		return nil, kerrors.NewInvalid("oauthClientAuthorization", authorization.Name, errs)
	}

	if err := s.registry.CreateClientAuthorization(authorization); err != nil {
		return nil, err
	}
	return s.Get(ctx, authorization.Name)
}

// Update modifies an existing client authorization
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	authorization, ok := obj.(*api.OAuthClientAuthorization)
	if !ok {
		return nil, false, fmt.Errorf("not an authorization: %#v", obj)
	}

	if errs := validation.ValidateClientAuthorization(authorization); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("oauthClientAuthorization", authorization.Name, errs)
	}

	oldauth, err := s.registry.GetClientAuthorization(authorization.Name)
	if err != nil {
		return nil, false, err
	}
	if errs := validation.ValidateClientAuthorizationUpdate(authorization, oldauth); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("oauthClientAuthorization", authorization.Name, errs)
	}

	if err := s.registry.UpdateClientAuthorization(authorization); err != nil {
		return nil, false, err
	}
	out, err := s.Get(ctx, authorization.Name)
	return out, false, err
}

// Delete asynchronously deletes an ClientAuthorization specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteClientAuthorization(id)
}
