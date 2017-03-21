package brokertemplateinstance

import (
	"fmt"

	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

// brokerTemplateInstanceStrategy implements behavior for Templates
type brokerTemplateInstanceStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating BrokerTemplateInstance
// objects via the REST API.
var Strategy = brokerTemplateInstanceStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is false for brokertemplateinstances.
func (brokerTemplateInstanceStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (brokerTemplateInstanceStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (brokerTemplateInstanceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (brokerTemplateInstanceStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Validate validates a new brokertemplateinstance.
func (brokerTemplateInstanceStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateBrokerTemplateInstance(obj.(*api.BrokerTemplateInstance))
}

// AllowCreateOnUpdate is false for brokertemplateinstances.
func (brokerTemplateInstanceStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (brokerTemplateInstanceStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (brokerTemplateInstanceStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBrokerTemplateInstanceUpdate(obj.(*api.BrokerTemplateInstance), old.(*api.BrokerTemplateInstance))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*api.BrokerTemplateInstance)
			if !ok {
				return nil, nil, fmt.Errorf("not a BrokerTemplateInstance")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.BrokerTemplateInstance) fields.Set {
	return api.BrokerTemplateInstanceToSelectableFields(obj)
}
