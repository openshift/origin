package egressnetworkpolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/apis/network/validation"
)

// enpStrategy implements behavior for EgressNetworkPolicies
type enpStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating EgressNetworkPolicy
// objects via the REST API.
var Strategy = enpStrategy{legacyscheme.Scheme}

var _ rest.GarbageCollectionDeleteStrategy = enpStrategy{}

func (enpStrategy) DefaultGarbageCollectionPolicy(ctx apirequest.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (enpStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is true for egress network policy
func (enpStrategy) NamespaceScoped() bool {
	return true
}

func (enpStrategy) GenerateName(base string) string {
	return base
}

func (enpStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (enpStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new egress network policy
func (enpStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicy(obj.(*networkapi.EgressNetworkPolicy))
}

// AllowCreateOnUpdate is false for egress network policies
func (enpStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (enpStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for a EgressNetworkPolicy
func (enpStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicyUpdate(obj.(*networkapi.EgressNetworkPolicy), old.(*networkapi.EgressNetworkPolicy))
}
