package client

import (
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPolicyBindingsReadOnlyInterface has methods to work with ClusterPolicyBindings resources in a namespace
type ClusterPolicyBindingsReadOnlyInterface interface {
	ReadOnlyClusterPolicyBindings() ReadOnlyClusterPolicyBindingInterface
}

// ReadOnlyClusterPolicyBindingInterface exposes methods on ClusterPolicyBindings resources
type ReadOnlyClusterPolicyBindingInterface interface {
	List(options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string) (*authorizationapi.ClusterPolicyBinding, error)
}
