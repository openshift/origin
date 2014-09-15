package useridentitymapping

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

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
func (s *REST) New() interface{} {
	return &api.UserIdentityMapping{}
}

// Get retrieves an UserIdentityMapping by id.
func (s *REST) Get(id string) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// List retrieves a list of UserIdentityMappings that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// Create is not supported for UserIdentityMappings
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// Update will create or update a UserIdentityMapping
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	mapping, ok := obj.(*api.UserIdentityMapping)
	if !ok {
		return nil, fmt.Errorf("not a user identity mapping: %#v", obj)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		updated, create, err := s.registry.CreateOrUpdateUserIdentityMapping(mapping)
		return apiserver.CreateOrUpdate{create, updated}, err
	}), nil
}

// Delete asynchronously deletes an UserIdentityMapping specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
