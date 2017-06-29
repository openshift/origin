package v1

import (
	v1 "github.com/openshift/origin/pkg/sdn/apis/network/v1"
	scheme "github.com/openshift/origin/pkg/sdn/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterNetworksGetter has a method to return a ClusterNetworkInterface.
// A group's client should implement this interface.
type ClusterNetworksGetter interface {
	ClusterNetworks(namespace string) ClusterNetworkInterface
}

// ClusterNetworkInterface has methods to work with ClusterNetwork resources.
type ClusterNetworkInterface interface {
	Create(*v1.ClusterNetwork) (*v1.ClusterNetwork, error)
	Update(*v1.ClusterNetwork) (*v1.ClusterNetwork, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.ClusterNetwork, error)
	List(opts meta_v1.ListOptions) (*v1.ClusterNetworkList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterNetwork, err error)
	ClusterNetworkExpansion
}

// clusterNetworks implements ClusterNetworkInterface
type clusterNetworks struct {
	client rest.Interface
	ns     string
}

// newClusterNetworks returns a ClusterNetworks
func newClusterNetworks(c *NetworkV1Client, namespace string) *clusterNetworks {
	return &clusterNetworks{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a clusterNetwork and creates it.  Returns the server's representation of the clusterNetwork, and an error, if there is any.
func (c *clusterNetworks) Create(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	result = &v1.ClusterNetwork{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("clusternetworks").
		Body(clusterNetwork).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterNetwork and updates it. Returns the server's representation of the clusterNetwork, and an error, if there is any.
func (c *clusterNetworks) Update(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	result = &v1.ClusterNetwork{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("clusternetworks").
		Name(clusterNetwork.Name).
		Body(clusterNetwork).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterNetwork and deletes it. Returns an error if one occurs.
func (c *clusterNetworks) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clusternetworks").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterNetworks) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the clusterNetwork, and returns the corresponding clusterNetwork object, and an error if there is any.
func (c *clusterNetworks) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterNetwork, err error) {
	result = &v1.ClusterNetwork{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clusternetworks").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterNetworks that match those selectors.
func (c *clusterNetworks) List(opts meta_v1.ListOptions) (result *v1.ClusterNetworkList, err error) {
	result = &v1.ClusterNetworkList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterNetworks.
func (c *clusterNetworks) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *clusterNetworks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterNetwork, err error) {
	result = &v1.ClusterNetwork{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("clusternetworks").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
