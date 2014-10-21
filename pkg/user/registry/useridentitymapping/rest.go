package useridentitymapping

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new UserIdentityMapping for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.UserIdentityMapping{}
}

// Get retrieves an UserIdentityMapping by id.
func (s *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	return nil, fmt.Errorf("not implemented")
}

// List retrieves a list of UserIdentityMappings that match selector.
func (s *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, fmt.Errorf("not implemented")
}

// Create is not supported for UserIdentityMappings
func (s *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("not implemented")
}

// Update will create or update a UserIdentityMapping
func (s *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	mapping, ok := obj.(*api.UserIdentityMapping)
	if !ok {
		return nil, fmt.Errorf("not a user identity mapping: %#v", obj)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		obj, created, err := s.registry.CreateOrUpdateUserIdentityMapping(mapping)
		return &apiserver.CreateOrUpdate{Created: created, Object: obj}, err
	}), nil
}

// Delete asynchronously deletes an UserIdentityMapping specified by its id.
func (s *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("not implemented")
}
