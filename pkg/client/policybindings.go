package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// PolicyBindingsNamespacer has methods to work with PolicyBinding resources in a namespace
type PolicyBindingsNamespacer interface {
	PolicyBindings(namespace string) PolicyBindingInterface
}

// PolicyBindingInterface exposes methods on PolicyBinding resources.
type PolicyBindingInterface interface {
	List(opts metav1.ListOptions) (*authorizationapi.PolicyBindingList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.PolicyBinding, error)
	Create(policyBinding *authorizationapi.PolicyBinding) (*authorizationapi.PolicyBinding, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type PolicyBindingsListerNamespacer interface {
	PolicyBindings(namespace string) PolicyBindingLister
}
type SyncedPolicyBindingsListerNamespacer interface {
	PolicyBindingsListerNamespacer
	LastSyncResourceVersion() string
}
type PolicyBindingLister interface {
	List(options metav1.ListOptions) (*authorizationapi.PolicyBindingList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.PolicyBinding, error)
}

// policyBindings implements PolicyBindingsNamespacer interface
type policyBindings struct {
	r  *Client
	ns string
}

// newPolicyBindings returns a policyBindings
func newPolicyBindings(c *Client, namespace string) *policyBindings {
	return &policyBindings{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of policyBindings that match the label and field selectors.
func (c *policyBindings) List(opts metav1.ListOptions) (result *authorizationapi.PolicyBindingList, err error) {
	result = &authorizationapi.PolicyBindingList{}
	err = c.r.Get().Namespace(c.ns).Resource("policyBindings").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

// Get returns information about a particular policyBinding and error if one occurs.
func (c *policyBindings) Get(name string, options metav1.GetOptions) (result *authorizationapi.PolicyBinding, err error) {
	result = &authorizationapi.PolicyBinding{}
	err = c.r.Get().Namespace(c.ns).Resource("policyBindings").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Create creates new policyBinding. Returns the server's representation of the policyBinding and error if one occurs.
func (c *policyBindings) Create(policyBinding *authorizationapi.PolicyBinding) (result *authorizationapi.PolicyBinding, err error) {
	result = &authorizationapi.PolicyBinding{}
	err = c.r.Post().Namespace(c.ns).Resource("policyBindings").Body(policyBinding).Do().Into(result)
	return
}

// Delete deletes a policyBinding, returns error if one occurs.
func (c *policyBindings) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("policyBindings").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested policyBindings
func (c *policyBindings) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Namespace(c.ns).Resource("policyBindings").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
