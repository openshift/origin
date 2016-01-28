package client

import (
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PolicyBindingsReadOnlyNamespacer has methods to work with PolicyBindings resources in a namespace
type PolicyBindingsReadOnlyNamespacer interface {
	ReadOnlyPolicyBindings(namespace string) ReadOnlyPolicyBindingInterface
}

// ReadOnlyPolicyBindingInterface exposes methods on PolicyBindings resources
type ReadOnlyPolicyBindingInterface interface {
	List(options *kapi.ListOptions) (*authorizationapi.PolicyBindingList, error)
	Get(name string) (*authorizationapi.PolicyBinding, error)
}
