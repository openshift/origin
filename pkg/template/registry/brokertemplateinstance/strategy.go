package brokertemplateinstance

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/apis/template/validation"
)

// brokerTemplateInstanceStrategy implements behavior for BrokerTemplateInstances
type brokerTemplateInstanceStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating BrokerTemplateInstance
// objects via the REST API.
var Strategy = brokerTemplateInstanceStrategy{kapi.Scheme, names.SimpleNameGenerator}

// NamespaceScoped is false for brokertemplateinstances.
func (brokerTemplateInstanceStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (brokerTemplateInstanceStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (brokerTemplateInstanceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (brokerTemplateInstanceStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new brokertemplateinstance.
func (brokerTemplateInstanceStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateBrokerTemplateInstance(obj.(*templateapi.BrokerTemplateInstance))
}

// AllowCreateOnUpdate is false for brokertemplateinstances.
func (brokerTemplateInstanceStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (brokerTemplateInstanceStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (brokerTemplateInstanceStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBrokerTemplateInstanceUpdate(obj.(*templateapi.BrokerTemplateInstance), old.(*templateapi.BrokerTemplateInstance))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*templateapi.BrokerTemplateInstance)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a BrokerTemplateInstance")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *templateapi.BrokerTemplateInstance) fields.Set {
	return templateapi.BrokerTemplateInstanceToSelectableFields(obj)
}
