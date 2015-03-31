package role

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for policies.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New creates a new Role object
func (r *REST) New() runtime.Object {
	return &authorizationapi.Role{}
}

func (*REST) NewList() runtime.Object {
	return &authorizationapi.RoleList{}
}

// List obtains a list of roles that match label.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	roles, err := r.registry.ListRoles(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return roles, err

}

// Get obtains the role specified by its name.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	role, err := r.registry.GetRole(ctx, name)
	if err != nil {
		return nil, err
	}
	return role, err
}

// Delete asynchronously deletes the role specified by its name.
func (r *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteRole(ctx, name)
}

// Create registers a given new Role inside the Policy instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	role, ok := obj.(*authorizationapi.Role)
	if !ok {
		return nil, fmt.Errorf("not a role: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &role.ObjectMeta) {
		return nil, kerrors.NewConflict("role", role.Namespace, fmt.Errorf("Role.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &role.ObjectMeta)
	if errs := validation.ValidateRole(role); len(errs) > 0 {
		return nil, kerrors.NewInvalid("role", role.Name, errs)
	}

	err := r.registry.CreateRole(ctx, role)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// Update replaces a given Role inside the Policy instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	role, ok := obj.(*authorizationapi.Role)
	if !ok {
		return nil, false, fmt.Errorf("not a role: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &role.ObjectMeta) {
		return nil, false, kerrors.NewConflict("role", role.Namespace, fmt.Errorf("Role.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRole(role); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("role", role.Name, errs)
	}

	err := r.registry.UpdateRole(ctx, role)
	if err != nil {
		return nil, false, err
	}
	return role, false, nil
}
