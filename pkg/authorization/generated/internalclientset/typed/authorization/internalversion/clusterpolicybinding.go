package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	scheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterPolicyBindingsGetter has a method to return a ClusterPolicyBindingInterface.
// A group's client should implement this interface.
type ClusterPolicyBindingsGetter interface {
	ClusterPolicyBindings() ClusterPolicyBindingInterface
}

// ClusterPolicyBindingInterface has methods to work with ClusterPolicyBinding resources.
type ClusterPolicyBindingInterface interface {
	Create(*authorization.ClusterPolicyBinding) (*authorization.ClusterPolicyBinding, error)
	Update(*authorization.ClusterPolicyBinding) (*authorization.ClusterPolicyBinding, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*authorization.ClusterPolicyBinding, error)
	List(opts v1.ListOptions) (*authorization.ClusterPolicyBindingList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicyBinding, err error)
	ClusterPolicyBindingExpansion
}

// clusterPolicyBindings implements ClusterPolicyBindingInterface
type clusterPolicyBindings struct {
	client rest.Interface
}

// newClusterPolicyBindings returns a ClusterPolicyBindings
func newClusterPolicyBindings(c *AuthorizationClient) *clusterPolicyBindings {
	return &clusterPolicyBindings{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterPolicyBinding, and returns the corresponding clusterPolicyBinding object, and an error if there is any.
func (c *clusterPolicyBindings) Get(name string, options v1.GetOptions) (result *authorization.ClusterPolicyBinding, err error) {
	result = &authorization.ClusterPolicyBinding{}
	err = c.client.Get().
		Resource("clusterpolicybindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterPolicyBindings that match those selectors.
func (c *clusterPolicyBindings) List(opts v1.ListOptions) (result *authorization.ClusterPolicyBindingList, err error) {
	result = &authorization.ClusterPolicyBindingList{}
	err = c.client.Get().
		Resource("clusterpolicybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterPolicyBindings.
func (c *clusterPolicyBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterpolicybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterPolicyBinding and creates it.  Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *clusterPolicyBindings) Create(clusterPolicyBinding *authorization.ClusterPolicyBinding) (result *authorization.ClusterPolicyBinding, err error) {
	result = &authorization.ClusterPolicyBinding{}
	err = c.client.Post().
		Resource("clusterpolicybindings").
		Body(clusterPolicyBinding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterPolicyBinding and updates it. Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *clusterPolicyBindings) Update(clusterPolicyBinding *authorization.ClusterPolicyBinding) (result *authorization.ClusterPolicyBinding, err error) {
	result = &authorization.ClusterPolicyBinding{}
	err = c.client.Put().
		Resource("clusterpolicybindings").
		Name(clusterPolicyBinding.Name).
		Body(clusterPolicyBinding).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterPolicyBinding and deletes it. Returns an error if one occurs.
func (c *clusterPolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterpolicybindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterPolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterpolicybindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterPolicyBinding.
func (c *clusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicyBinding, err error) {
	result = &authorization.ClusterPolicyBinding{}
	err = c.client.Patch(pt).
		Resource("clusterpolicybindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
