package v1

import (
	v1 "github.com/openshift/api/authorization/v1"
	scheme "github.com/openshift/client-go/authorization/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Create(*v1.ClusterPolicyBinding) (*v1.ClusterPolicyBinding, error)
	Update(*v1.ClusterPolicyBinding) (*v1.ClusterPolicyBinding, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.ClusterPolicyBinding, error)
	List(opts meta_v1.ListOptions) (*v1.ClusterPolicyBindingList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterPolicyBinding, err error)
	ClusterPolicyBindingExpansion
}

// clusterPolicyBindings implements ClusterPolicyBindingInterface
type clusterPolicyBindings struct {
	client rest.Interface
}

// newClusterPolicyBindings returns a ClusterPolicyBindings
func newClusterPolicyBindings(c *AuthorizationV1Client) *clusterPolicyBindings {
	return &clusterPolicyBindings{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterPolicyBinding, and returns the corresponding clusterPolicyBinding object, and an error if there is any.
func (c *clusterPolicyBindings) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterPolicyBinding, err error) {
	result = &v1.ClusterPolicyBinding{}
	err = c.client.Get().
		Resource("clusterpolicybindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterPolicyBindings that match those selectors.
func (c *clusterPolicyBindings) List(opts meta_v1.ListOptions) (result *v1.ClusterPolicyBindingList, err error) {
	result = &v1.ClusterPolicyBindingList{}
	err = c.client.Get().
		Resource("clusterpolicybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterPolicyBindings.
func (c *clusterPolicyBindings) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterpolicybindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterPolicyBinding and creates it.  Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *clusterPolicyBindings) Create(clusterPolicyBinding *v1.ClusterPolicyBinding) (result *v1.ClusterPolicyBinding, err error) {
	result = &v1.ClusterPolicyBinding{}
	err = c.client.Post().
		Resource("clusterpolicybindings").
		Body(clusterPolicyBinding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterPolicyBinding and updates it. Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *clusterPolicyBindings) Update(clusterPolicyBinding *v1.ClusterPolicyBinding) (result *v1.ClusterPolicyBinding, err error) {
	result = &v1.ClusterPolicyBinding{}
	err = c.client.Put().
		Resource("clusterpolicybindings").
		Name(clusterPolicyBinding.Name).
		Body(clusterPolicyBinding).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterPolicyBinding and deletes it. Returns an error if one occurs.
func (c *clusterPolicyBindings) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterpolicybindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterPolicyBindings) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterpolicybindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterPolicyBinding.
func (c *clusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterPolicyBinding, err error) {
	result = &v1.ClusterPolicyBinding{}
	err = c.client.Patch(pt).
		Resource("clusterpolicybindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
