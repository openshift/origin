package rolebinding

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// TODO add get and list

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for policies.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New creates a new RoleBinding object
func (r *REST) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}

// Delete asynchronously deletes the PolicyBinding specified by its name.
func (r *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteRoleBinding(ctx, name)
}

// Create registers a given new RoleBinding inside the PolicyBinding instance to r.bindingRegistry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, fmt.Errorf("not a roleBinding: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &roleBinding.ObjectMeta) {
		return nil, kerrors.NewConflict("roleBinding", roleBinding.Namespace, fmt.Errorf("RoleBinding.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &roleBinding.ObjectMeta)
	if errs := validation.ValidateRoleBinding(roleBinding); len(errs) > 0 {
		return nil, kerrors.NewInvalid("roleBinding", roleBinding.Name, errs)
	}

	err := r.registry.CreateRoleBinding(ctx, roleBinding)
	if err != nil {
		return nil, err
	}
	return roleBinding, nil
}

// Update replaces a given RoleBinding inside the PolicyBinding instance with an existing instance in r.bindingRegistry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, false, fmt.Errorf("not a roleBinding: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &roleBinding.ObjectMeta) {
		return nil, false, kerrors.NewConflict("roleBinding", roleBinding.Namespace, fmt.Errorf("RoleBinding.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRoleBinding(roleBinding); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("roleBinding", roleBinding.Name, errs)
	}

	err := r.registry.UpdateRoleBinding(ctx, roleBinding)
	if err != nil {
		return nil, false, err
	}
	return roleBinding, false, nil
}
