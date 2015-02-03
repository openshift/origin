package useridentitymapping

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new UserIdentityMapping for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.UserIdentityMapping{}
}

// Get retrieves an UserIdentityMapping by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return s.registry.GetUserIdentityMapping(id)
}

// Update will create or update a UserIdentityMapping
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	mapping, ok := obj.(*api.UserIdentityMapping)
	if !ok {
		return nil, fmt.Errorf("not a user identity mapping: %#v", obj)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		obj, _, err := s.registry.CreateOrUpdateUserIdentityMapping(mapping)
		// TODO: return created status
		// return &apiserver.CreateOrUpdate{Created: created, Object: obj}, err
		return obj, err
	}), nil
}
