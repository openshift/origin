package clusterpolicy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/validation"
)

// strategy implements behavior for nodes
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating ClusterPolicy objects.
var Strategy = strategy{kapi.Scheme}

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
func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	policy := obj.(*authorizationapi.ClusterPolicy)

	policy.Name = authorizationapi.PolicyName
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	_ = obj.(*authorizationapi.ClusterPolicy)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policy.
func (strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterPolicy(obj.(*authorizationapi.ClusterPolicy))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterPolicyUpdate(obj.(*authorizationapi.ClusterPolicy), old.(*authorizationapi.ClusterPolicy))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	policy, ok := obj.(*authorizationapi.ClusterPolicy)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a ClusterPolicy")
	}
	return labels.Set(policy.ObjectMeta.Labels), authorizationapi.ClusterPolicyToSelectableFields(policy), policy.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
