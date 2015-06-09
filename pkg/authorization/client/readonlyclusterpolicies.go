package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPoliciesReadOnlyInterface has methods to work with ClusterPolicies resources in a namespace
type ClusterPoliciesReadOnlyInterface interface {
	ReadOnlyClusterPolicies() ReadOnlyClusterPolicyInterface
}

// ReadOnlyClusterPolicyInterface exposes methods on ClusterPolicies resources
type ReadOnlyClusterPolicyInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
}
