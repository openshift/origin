package client

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PoliciesReadOnlyNamespacer has methods to work with Policies resources in a namespace
type PoliciesReadOnlyNamespacer interface {
	ReadOnlyPolicies(namespace string) ReadOnlyPolicyInterface
}

// ReadOnlyPolicyInterface exposes methods on Policies resources
type ReadOnlyPolicyInterface interface {
	List(options *unversioned.ListOptions) (*authorizationapi.PolicyList, error)
	Get(name string) (*authorizationapi.Policy, error)
}
