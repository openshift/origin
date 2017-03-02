package rolebindingrestriction

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

type strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

var Strategy = strategy{kapi.Scheme, kapi.SimpleNameGenerator}

func (strategy) NamespaceScoped() bool {
	return true
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	_ = obj.(*authorizationapi.RoleBindingRestriction)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	_ = obj.(*authorizationapi.RoleBindingRestriction)
	_ = old.(*authorizationapi.RoleBindingRestriction)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateRoleBindingRestriction(obj.(*authorizationapi.RoleBindingRestriction))
}

func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateRoleBindingRestrictionUpdate(obj.(*authorizationapi.RoleBindingRestriction), old.(*authorizationapi.RoleBindingRestriction))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	rbr, ok := obj.(*authorizationapi.RoleBindingRestriction)
	if !ok {
		return nil, nil, fmt.Errorf("not a RoleBindingRestriction")
	}
	return labels.Set(rbr.ObjectMeta.Labels), authorizationapi.RoleBindingRestrictionToSelectableFields(rbr), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
