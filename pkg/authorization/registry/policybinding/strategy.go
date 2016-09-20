package policybinding

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// strategy implements behavior for nodes
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

// NamespaceScoped is true for policybindings.
func (strategy) NamespaceScoped() bool {
	return true
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
func (s strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	binding := obj.(*authorizationapi.PolicyBinding)

	s.scrubBindingRefs(binding)
	// force a delimited name, just in case we someday allow a reference to a global object that won't have a namespace.  We'll end up with a name like ":default".
	// ":" is not in the value space of namespaces, so no escaping is necessary
	binding.Name = authorizationapi.GetPolicyBindingName(binding.PolicyRef.Namespace)
}

// scrubBindingRefs discards pieces of the object references that we don't respect to avoid confusion.
func (s strategy) scrubBindingRefs(binding *authorizationapi.PolicyBinding) {
	binding.PolicyRef = kapi.ObjectReference{Namespace: binding.PolicyRef.Namespace, Name: authorizationapi.PolicyName}

	for roleBindingKey, roleBinding := range binding.RoleBindings {
		roleBinding.RoleRef = kapi.ObjectReference{Namespace: binding.PolicyRef.Namespace, Name: roleBinding.RoleRef.Name}
		binding.RoleBindings[roleBindingKey] = roleBinding
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (s strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	binding := obj.(*authorizationapi.PolicyBinding)

	s.scrubBindingRefs(binding)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policyBinding.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateLocalPolicyBinding(obj.(*authorizationapi.PolicyBinding))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateLocalPolicyBindingUpdate(obj.(*authorizationapi.PolicyBinding), old.(*authorizationapi.PolicyBinding))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			policyBinding, ok := obj.(*authorizationapi.PolicyBinding)
			if !ok {
				return nil, nil, fmt.Errorf("not a policyBinding")
			}
			return labels.Set(policyBinding.ObjectMeta.Labels), authorizationapi.PolicyBindingToSelectableFields(policyBinding), nil
		},
	}
}

func NewEmptyPolicyBinding(namespace, policyNamespace, policyBindingName string) *authorizationapi.PolicyBinding {
	binding := &authorizationapi.PolicyBinding{}
	binding.Name = policyBindingName
	binding.Namespace = namespace
	binding.CreationTimestamp = unversioned.Now()
	binding.LastModified = binding.CreationTimestamp
	binding.PolicyRef = kapi.ObjectReference{Name: authorizationapi.PolicyName, Namespace: policyNamespace}
	binding.RoleBindings = make(map[string]*authorizationapi.RoleBinding)

	return binding
}
