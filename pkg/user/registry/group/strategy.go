package group

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// groupStrategy implements behavior for Groups
type groupStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Group
// objects via the REST API.
var Strategy = groupStrategy{kapi.Scheme}

func (groupStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for groups
func (groupStrategy) NamespaceScoped() bool {
	return false
}

func (groupStrategy) GenerateName(base string) string {
	return base
}

func (groupStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Validate validates a new group
func (groupStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateGroup(obj.(*api.Group))
}

// AllowCreateOnUpdate is false for groups
func (groupStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (groupStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (groupStrategy) Canonicalize(obj runtime.Object) {
}

// ValidateUpdate is the default update validation for an end group.
func (groupStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateGroupUpdate(obj.(*api.Group), old.(*api.Group))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*api.Group)
			if !ok {
				return nil, nil, fmt.Errorf("not a Group")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.Group) fields.Set {
	return api.GroupToSelectableFields(obj)
}
