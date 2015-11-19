package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PolicyBindingsReadOnlyNamespacer has methods to work with PolicyBindings resources in a namespace
type PolicyBindingsReadOnlyNamespacer interface {
	ReadOnlyPolicyBindings(namespace string) ReadOnlyPolicyBindingInterface
}

// ReadOnlyPolicyBindingInterface exposes methods on PolicyBindings resources
type ReadOnlyPolicyBindingInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error)
	Get(name string) (*authorizationapi.PolicyBinding, error)
}
