package role

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// strategy implements behavior for nodes
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Role objects.
var Strategy = strategy{kapi.Scheme}

// NamespaceScoped is false for policies.
func (strategy) NamespaceScoped() bool {
	return true
}

// AllowCreateOnUpdate is false for policies.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(obj runtime.Object) {
	_ = obj.(*authorizationapi.Role)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(obj, old runtime.Object) {
	_ = obj.(*authorizationapi.Role)
}

// Validate validates a new role.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateRole(obj.(*authorizationapi.Role))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateRoleUpdate(obj.(*authorizationapi.Role), old.(*authorizationapi.Role))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			role, ok := obj.(*authorizationapi.Role)
			if !ok {
				return nil, nil, fmt.Errorf("not a role")
			}
			return labels.Set(role.ObjectMeta.Labels), SelectableFields(role), nil
		},
	}
}

// SelectableFields returns a label set that represents the object
func SelectableFields(role *authorizationapi.Role) fields.Set {
	return fields.Set{
		"name": role.Name,
	}
}
