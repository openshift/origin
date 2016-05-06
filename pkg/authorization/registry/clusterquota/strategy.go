package clusterquota

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/authorization/api"
)

// strategy implements behavior for nodes
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Policy objects.
var Strategy = strategy{kapi.Scheme}

// NamespaceScoped is true for policies.
func (strategy) NamespaceScoped() bool {
	return false
}

// AllowCreateOnUpdate is false for policies.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(obj runtime.Object) {
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(obj, old runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policy.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return nil
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			quota, ok := obj.(*api.ClusterResourceQuota)
			if !ok {
				return nil, nil, fmt.Errorf("not a ClusterResourceQuota")
			}
			return labels.Set(quota.ObjectMeta.Labels), nil, nil
		},
	}
}

type resourcequotaStatusStrategy struct {
	strategy
}

var StatusStrategy = resourcequotaStatusStrategy{Strategy}

func (resourcequotaStatusStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newResourcequota := obj.(*api.ClusterResourceQuota)
	oldResourcequota := old.(*api.ClusterResourceQuota)
	newResourcequota.Spec = oldResourcequota.Spec
}

func (resourcequotaStatusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

// MatchResourceQuota returns a generic matcher for a given label and field selector.
func MatchResourceQuota(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		resourcequotaObj, ok := obj.(*api.ClusterResourceQuota)
		if !ok {
			return false, fmt.Errorf("not a resourcequota")
		}
		fields := ResourceQuotaToSelectableFields(resourcequotaObj)
		return label.Matches(labels.Set(resourcequotaObj.Labels)) && field.Matches(fields), nil
	})
}

// ResourceQuotaToSelectableFields returns a label set that represents the object
func ResourceQuotaToSelectableFields(resourcequota *api.ClusterResourceQuota) labels.Set {
	return labels.Set(generic.ObjectMetaFieldsSet(resourcequota.ObjectMeta, true))
}
