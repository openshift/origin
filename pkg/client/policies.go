package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PoliciesNamespacer has methods to work with Policy resources in a namespace
type PoliciesNamespacer interface {
	Policies(namespace string) PolicyInterface
}

// PolicyInterface exposes methods on Policy resources.
type PolicyInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyList, error)
	Get(name string) (*authorizationapi.Policy, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// policies implements PoliciesNamespacer interface
type policies struct {
	r  *Client
	ns string
}

// newPolicies returns a policies
func newPolicies(c *Client, namespace string) *policies {
	return &policies{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of policies that match the label and field selectors.
func (c *policies) List(label labels.Selector, field fields.Selector) (result *authorizationapi.PolicyList, err error) {
	result = &authorizationapi.PolicyList{}
	err = c.r.Get().Namespace(c.ns).Resource("policies").LabelsSelectorParam(label).FieldsSelectorParam(field).Do().Into(result)
	return
}

// Get returns information about a particular policy and error if one occurs.
func (c *policies) Get(name string) (result *authorizationapi.Policy, err error) {
	result = &authorizationapi.Policy{}
	err = c.r.Get().Namespace(c.ns).Resource("policies").Name(name).Do().Into(result)
	return
}

// Delete deletes a policy, returns error if one occurs.
func (c *policies) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("policies").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested policies
func (c *policies) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Namespace(c.ns).Resource("policies").Param("resourceVersion", resourceVersion).LabelsSelectorParam(label).FieldsSelectorParam(field).Watch()
}
