package useridentitymapping

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
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
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	mapping, ok := obj.(*api.UserIdentityMapping)
	if !ok {
		return nil, false, fmt.Errorf("not a user identity mapping: %#v", obj)
	}
	if errs := validation.ValidateUserIdentityMapping(mapping); len(errs) > 0 {
		return nil, false, errors.NewInvalid("userIdentityMapping", mapping.Name, errs)
	}
	return s.registry.CreateOrUpdateUserIdentityMapping(mapping)
}
