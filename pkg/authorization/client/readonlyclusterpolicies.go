package client

import (
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPoliciesReadOnlyInterface has methods to work with ClusterPolicies resources in a namespace
type ClusterPoliciesReadOnlyInterface interface {
	ReadOnlyClusterPolicies() ReadOnlyClusterPolicyInterface
}

// ReadOnlyClusterPolicyInterface exposes methods on ClusterPolicies resources
type ReadOnlyClusterPolicyInterface interface {
	List(options *kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
}
