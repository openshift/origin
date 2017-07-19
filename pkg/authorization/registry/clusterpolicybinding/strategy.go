package clusterpolicybinding

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

// Strategy is the default logic that applies when creating and updating ClusterPolicyBinding objects.
var Strategy = strategy{kapi.Scheme}

func (strategy) NamespaceScoped() bool {
	return false
}

// AllowCreateOnUpdate is false for policybindings.
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
func (s strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	binding := obj.(*authorizationapi.ClusterPolicyBinding)

	s.scrubBindingRefs(binding)
	// force a delimited name, just in case we someday allow a reference to a global object that won't have a namespace.  We'll end up with a name like ":default".
	// ":" is not in the value space of namespaces, so no escaping is necessary
	binding.Name = authorizationapi.GetPolicyBindingName(binding.PolicyRef.Namespace)
}

// scrubBindingRefs discards pieces of the object references that we don't respect to avoid confusion.
func (s strategy) scrubBindingRefs(binding *authorizationapi.ClusterPolicyBinding) {
	binding.PolicyRef = kapi.ObjectReference{Namespace: binding.PolicyRef.Namespace, Name: authorizationapi.PolicyName}
	binding.PolicyRef.Namespace = ""

	for roleBindingKey, roleBinding := range binding.RoleBindings {
		roleBinding.RoleRef = kapi.ObjectReference{Namespace: binding.PolicyRef.Namespace, Name: roleBinding.RoleRef.Name}
		binding.RoleBindings[roleBindingKey] = roleBinding
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (s strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	binding := obj.(*authorizationapi.ClusterPolicyBinding)

	s.scrubBindingRefs(binding)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policyBinding.
func (strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterPolicyBinding(obj.(*authorizationapi.ClusterPolicyBinding))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterPolicyBindingUpdate(obj.(*authorizationapi.ClusterPolicyBinding), old.(*authorizationapi.ClusterPolicyBinding))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	policyBinding, ok := obj.(*authorizationapi.ClusterPolicyBinding)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a ClusterPolicyBinding")
	}
	return labels.Set(policyBinding.ObjectMeta.Labels), authorizationapi.ClusterPolicyBindingToSelectableFields(policyBinding), policyBinding.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
