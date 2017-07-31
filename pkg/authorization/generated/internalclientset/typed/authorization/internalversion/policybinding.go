package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	scheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PolicyBindingsGetter has a method to return a PolicyBindingInterface.
// A group's client should implement this interface.
type PolicyBindingsGetter interface {
	PolicyBindings(namespace string) PolicyBindingInterface
}

// PolicyBindingInterface has methods to work with PolicyBinding resources.
type PolicyBindingInterface interface {
	Create(*authorization.PolicyBinding) (*authorization.PolicyBinding, error)
	Update(*authorization.PolicyBinding) (*authorization.PolicyBinding, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*authorization.PolicyBinding, error)
	List(opts v1.ListOptions) (*authorization.PolicyBindingList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.PolicyBinding, err error)
	PolicyBindingExpansion
}

// policyBindings implements PolicyBindingInterface
type policyBindings struct {
	client rest.Interface
	ns     string
}

// newPolicyBindings returns a PolicyBindings
func newPolicyBindings(c *AuthorizationClient, namespace string) *policyBindings {
	return &policyBindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the policyBinding, and returns the corresponding policyBinding object, and an error if there is any.
func (c *policyBindings) Get(name string, options v1.GetOptions) (result *authorization.PolicyBinding, err error) {
	result = &authorization.PolicyBinding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policybindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PolicyBindings that match those selectors.
func (c *policyBindings) List(opts v1.ListOptions) (result *authorization.PolicyBindingList, err error) {
	result = &authorization.PolicyBindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested policyBindings.
func (c *policyBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("policybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a policyBinding and creates it.  Returns the server's representation of the policyBinding, and an error, if there is any.
func (c *policyBindings) Create(policyBinding *authorization.PolicyBinding) (result *authorization.PolicyBinding, err error) {
	result = &authorization.PolicyBinding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("policybindings").
		Body(policyBinding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a policyBinding and updates it. Returns the server's representation of the policyBinding, and an error, if there is any.
func (c *policyBindings) Update(policyBinding *authorization.PolicyBinding) (result *authorization.PolicyBinding, err error) {
	result = &authorization.PolicyBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("policybindings").
		Name(policyBinding.Name).
		Body(policyBinding).
		Do().
		Into(result)
	return
}

// Delete takes name of the policyBinding and deletes it. Returns an error if one occurs.
func (c *policyBindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policybindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *policyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policybindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched policyBinding.
func (c *policyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.PolicyBinding, err error) {
	result = &authorization.PolicyBinding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("policybindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
