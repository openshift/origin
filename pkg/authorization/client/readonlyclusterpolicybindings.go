package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

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
