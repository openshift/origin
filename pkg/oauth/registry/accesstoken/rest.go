package accesstoken

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/oauth/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new AccessToken for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.AccessToken{}
}

// Get retrieves an AccessToken by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	token, err := s.registry.GetAccessToken(id)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// List retrieves a list of AccessTokens that match selector.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	tokens, err := s.registry.ListAccessTokens(selector)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// Create registers the given AccessToken.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	token, ok := obj.(*api.AccessToken)
	if !ok {
		return nil, fmt.Errorf("not an token: %#v", obj)
	}

	token.CreationTimestamp = util.Now()

	// if errs := validation.ValidateAccessToken(token); len(errs) > 0 {
	// 	return nil, errors.NewInvalid("token", token.Name, errs)
	// }

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		if err := s.registry.CreateAccessToken(token); err != nil {
			return nil, err
		}
		return s.Get(ctx, token.Name)
	}), nil
}

// Update is not supported for AccessTokens, as they are immutable.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, fmt.Errorf("AccessTokens may not be changed.")
}

// Delete asynchronously deletes an AccessToken specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteAccessToken(id)
	}), nil
}
