package rolebinding

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// strategy implements behavior for nodes
type strategy struct {
	namespaced bool

	runtime.ObjectTyper
}

var ClusterStrategy = strategy{false, kapi.Scheme}
var LocalStrategy = strategy{true, kapi.Scheme}

// NamespaceScoped is false for rolebindings.
func (s strategy) NamespaceScoped() bool {
	return s.namespaced
}

// AllowCreateOnUpdate is false for rolebindings.
func (s strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return true
}

func (s strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	_ = obj.(*authorizationapi.RoleBinding)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (s strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	_ = obj.(*authorizationapi.RoleBinding)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new role.
func (s strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateRoleBinding(obj.(*authorizationapi.RoleBinding), s.namespaced)
}

// ValidateUpdate is the default update validation for an end user.
func (s strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateRoleBindingUpdate(obj.(*authorizationapi.RoleBinding), old.(*authorizationapi.RoleBinding), s.namespaced)
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			roleBinding, ok := obj.(*authorizationapi.RoleBinding)
			if !ok {
				return nil, nil, fmt.Errorf("not a rolebinding")
			}
			return labels.Set(roleBinding.ObjectMeta.Labels), authorizationapi.RoleBindingToSelectableFields(roleBinding), nil
		},
	}
}
