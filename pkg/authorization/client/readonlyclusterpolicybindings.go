package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPolicyBindingsReadOnlyInterface has methods to work with ClusterPolicyBindings resources in a namespace
type ClusterPolicyBindingsReadOnlyInterface interface {
	ReadOnlyClusterPolicyBindings() ReadOnlyClusterPolicyBindingInterface
}

// ReadOnlyClusterPolicyBindingInterface exposes methods on ClusterPolicyBindings resources
type ReadOnlyClusterPolicyBindingInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string) (*authorizationapi.ClusterPolicyBinding, error)
}
