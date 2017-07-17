package clusterresourcequota

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/apis/quota/validation"
)

type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

func (strategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

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

func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	quota := obj.(*quotaapi.ClusterResourceQuota)
	quota.Status = quotaapi.ClusterResourceQuotaStatus{}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	curr := obj.(*quotaapi.ClusterResourceQuota)
	prev := old.(*quotaapi.ClusterResourceQuota)

	curr.Status = prev.Status
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuota(obj.(*quotaapi.ClusterResourceQuota))
}

func (strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuotaUpdate(obj.(*quotaapi.ClusterResourceQuota), old.(*quotaapi.ClusterResourceQuota))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	quota, ok := obj.(*quotaapi.ClusterResourceQuota)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a ClusterResourceQuota")
	}
	return labels.Set(quota.ObjectMeta.Labels), quotaapi.ClusterResourceQuotaToSelectableFields(quota), quota.Initializers != nil, nil
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

func (statusStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

func (statusStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	curr := obj.(*quotaapi.ClusterResourceQuota)
	prev := old.(*quotaapi.ClusterResourceQuota)

	curr.Spec = prev.Spec
}

func (statusStrategy) Canonicalize(obj runtime.Object) {
}

func (statusStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuota(obj.(*quotaapi.ClusterResourceQuota))
}

func (statusStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterResourceQuotaUpdate(obj.(*quotaapi.ClusterResourceQuota), old.(*quotaapi.ClusterResourceQuota))
}
