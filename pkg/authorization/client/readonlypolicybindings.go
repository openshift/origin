package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

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
