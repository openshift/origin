package v1

import (
	v1 "github.com/openshift/api/authorization/v1"
	scheme "github.com/openshift/client-go/authorization/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PoliciesGetter has a method to return a PolicyInterface.
// A group's client should implement this interface.
type PoliciesGetter interface {
	Policies(namespace string) PolicyInterface
}

// PolicyInterface has methods to work with Policy resources.
type PolicyInterface interface {
	Create(*v1.Policy) (*v1.Policy, error)
	Update(*v1.Policy) (*v1.Policy, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Policy, error)
	List(opts meta_v1.ListOptions) (*v1.PolicyList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Policy, err error)
	PolicyExpansion
}

// policies implements PolicyInterface
type policies struct {
	client rest.Interface
	ns     string
}

// newPolicies returns a Policies
func newPolicies(c *AuthorizationV1Client, namespace string) *policies {
	return &policies{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the policy, and returns the corresponding policy object, and an error if there is any.
func (c *policies) Get(name string, options meta_v1.GetOptions) (result *v1.Policy, err error) {
	result = &v1.Policy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Policies that match those selectors.
func (c *policies) List(opts meta_v1.ListOptions) (result *v1.PolicyList, err error) {
	result = &v1.PolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *policies) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a policy and creates it.  Returns the server's representation of the policy, and an error, if there is any.
func (c *policies) Create(policy *v1.Policy) (result *v1.Policy, err error) {
	result = &v1.Policy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("policies").
		Body(policy).
		Do().
		Into(result)
	return
}

// Update takes the representation of a policy and updates it. Returns the server's representation of the policy, and an error, if there is any.
func (c *policies) Update(policy *v1.Policy) (result *v1.Policy, err error) {
	result = &v1.Policy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("policies").
		Name(policy.Name).
		Body(policy).
		Do().
		Into(result)
	return
}

// Delete takes name of the policy and deletes it. Returns an error if one occurs.
func (c *policies) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policies").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *policies) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched policy.
func (c *policies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Policy, err error) {
	result = &v1.Policy{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("policies").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
