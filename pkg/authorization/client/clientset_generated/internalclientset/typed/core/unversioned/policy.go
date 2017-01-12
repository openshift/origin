package unversioned

import (
	api "github.com/openshift/origin/pkg/authorization/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// PoliciesGetter has a method to return a PolicyInterface.
// A group's client should implement this interface.
type PoliciesGetter interface {
	Policies(namespace string) PolicyInterface
}

// PolicyInterface has methods to work with Policy resources.
type PolicyInterface interface {
	Create(*api.Policy) (*api.Policy, error)
	Update(*api.Policy) (*api.Policy, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.Policy, error)
	List(opts pkg_api.ListOptions) (*api.PolicyList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Policy, err error)
	PolicyExpansion
}

// policies implements PolicyInterface
type policies struct {
	client *CoreClient
	ns     string
}

// newPolicies returns a Policies
func newPolicies(c *CoreClient, namespace string) *policies {
	return &policies{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a policy and creates it.  Returns the server's representation of the policy, and an error, if there is any.
func (c *policies) Create(policy *api.Policy) (result *api.Policy, err error) {
	result = &api.Policy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("policies").
		Body(policy).
		Do().
		Into(result)
	return
}

// Update takes the representation of a policy and updates it. Returns the server's representation of the policy, and an error, if there is any.
func (c *policies) Update(policy *api.Policy) (result *api.Policy, err error) {
	result = &api.Policy{}
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
func (c *policies) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policies").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *policies) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the policy, and returns the corresponding policy object, and an error if there is any.
func (c *policies) Get(name string) (result *api.Policy, err error) {
	result = &api.Policy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policies").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Policies that match those selectors.
func (c *policies) List(opts pkg_api.ListOptions) (result *api.PolicyList, err error) {
	result = &api.PolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *policies) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("policies").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched policy.
func (c *policies) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Policy, err error) {
	result = &api.Policy{}
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
