package client

import (
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ReadOnlyPolicyClient exposes List() and Get() for policies and bindings along with the the last synced resource version
type ReadOnlyPolicyClient interface {
	// Embedded interfaces to allow read-only access to policies and bindings on project and cluster level
	PoliciesReadOnlyNamespacer
	ClusterPoliciesReadOnlyInterface
	PolicyBindingsReadOnlyNamespacer
	ClusterPolicyBindingsReadOnlyInterface

	// Returns the last synced resource version for re-sync sanity checks
	LastSyncResourceVersion() string

	// Methods that enable the ReadOnlyPolicyClient to conform to rulevalidation.PolicyGetter and rulevalidation.BindingLister interfaces
	GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error)
	ListPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.PolicyBindingList, error)
	GetClusterPolicy(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicy, error)
	ListClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
}
