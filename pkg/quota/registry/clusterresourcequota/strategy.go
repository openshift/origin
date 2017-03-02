package clusterresourcequota

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/quota/api/validation"
)

type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	quota := obj.(*api.ClusterResourceQuota)
	quota.Status = api.ClusterResourceQuotaStatus{}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	curr := obj.(*api.ClusterResourceQuota)
	prev := old.(*api.ClusterResourceQuota)

	curr.Status = prev.Status
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuota(obj.(*api.ClusterResourceQuota))
}

func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuotaUpdate(obj.(*api.ClusterResourceQuota), old.(*api.ClusterResourceQuota))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	quota, ok := obj.(*api.ClusterResourceQuota)
	if !ok {
		return nil, nil, fmt.Errorf("not a ClusterResourceQuota")
	}
	return labels.Set(quota.ObjectMeta.Labels), api.ClusterResourceQuotaToSelectableFields(quota), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

type statusStrategy struct {
	runtime.ObjectTyper
}

var StatusStrategy = statusStrategy{kapi.Scheme}

func (statusStrategy) NamespaceScoped() bool {
	return false
}

func (statusStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (statusStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (statusStrategy) GenerateName(base string) string {
	return base
}

func (statusStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

func (statusStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	curr := obj.(*api.ClusterResourceQuota)
	prev := old.(*api.ClusterResourceQuota)

	curr.Spec = prev.Spec
}

func (statusStrategy) Canonicalize(obj runtime.Object) {
}

func (statusStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuota(obj.(*api.ClusterResourceQuota))
}

func (statusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuotaUpdate(obj.(*api.ClusterResourceQuota), old.(*api.ClusterResourceQuota))
}
