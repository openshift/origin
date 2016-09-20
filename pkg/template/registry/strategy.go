package registry

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
)

// templateStrategy implements behavior for Templates
type templateStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Template
// objects via the REST API.
var Strategy = templateStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is true for templates.
func (templateStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (templateStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {}

// Canonicalize normalizes the object after validation.
func (templateStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (templateStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Validate validates a new template.
func (templateStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateTemplate(obj.(*api.Template))
}

// AllowCreateOnUpdate is false for templates.
func (templateStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (templateStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (templateStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateTemplateUpdate(obj.(*api.Template), old.(*api.Template))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*api.Template)
			if !ok {
				return nil, nil, fmt.Errorf("not a Template")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.Template) fields.Set {
	return api.TemplateToSelectableFields(obj)
}
