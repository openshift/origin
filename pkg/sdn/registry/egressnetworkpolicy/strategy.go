package egressnetworkpolicy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/api/validation"
)

// enpStrategy implements behavior for EgressNetworkPolicies
type enpStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating EgressNetworkPolicy
// objects via the REST API.
var Strategy = enpStrategy{kapi.Scheme}

func (enpStrategy) PrepareForUpdate(obj, old runtime.Object) {}

// NamespaceScoped is true for egress network policy
func (enpStrategy) NamespaceScoped() bool {
	return true
}

func (enpStrategy) GenerateName(base string) string {
	return base
}

func (enpStrategy) PrepareForCreate(obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (enpStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new egress network policy
func (enpStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicy(obj.(*api.EgressNetworkPolicy))
}

// AllowCreateOnUpdate is false for egress network policies
func (enpStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (enpStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for a EgressNetworkPolicy
func (enpStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicyUpdate(obj.(*api.EgressNetworkPolicy), old.(*api.EgressNetworkPolicy))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		network, ok := obj.(*api.EgressNetworkPolicy)
		if !ok {
			return false, fmt.Errorf("not an EgressNetworkPolicy")
		}
		fields := api.EgressNetworkPolicyToSelectableFields(network)
		return label.Matches(labels.Set(network.Labels)) && field.Matches(fields), nil
	})
}
