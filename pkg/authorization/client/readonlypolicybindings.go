package client

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PolicyBindingsReadOnlyNamespacer has methods to work with PolicyBindings resources in a namespace
type PolicyBindingsReadOnlyNamespacer interface {
	ReadOnlyPolicyBindings(namespace string) ReadOnlyPolicyBindingInterface
}

// ReadOnlyPolicyBindingInterface exposes methods on PolicyBindings resources
type ReadOnlyPolicyBindingInterface interface {
	List(options *unversioned.ListOptions) (*authorizationapi.PolicyBindingList, error)
	Get(name string) (*authorizationapi.PolicyBinding, error)
}
