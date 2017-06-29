package internalversion

import (
	network "github.com/openshift/origin/pkg/sdn/apis/network"
	scheme "github.com/openshift/origin/pkg/sdn/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Create(*network.ClusterNetwork) (*network.ClusterNetwork, error)
	Update(*network.ClusterNetwork) (*network.ClusterNetwork, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*network.ClusterNetwork, error)
	List(opts v1.ListOptions) (*network.ClusterNetworkList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.ClusterNetwork, err error)
	ClusterNetworkExpansion
}

// clusterNetworks implements ClusterNetworkInterface
type clusterNetworks struct {
	client rest.Interface
	ns     string
}

// newClusterNetworks returns a ClusterNetworks
func newClusterNetworks(c *NetworkClient, namespace string) *clusterNetworks {
	return &clusterNetworks{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a clusterNetwork and creates it.  Returns the server's representation of the clusterNetwork, and an error, if there is any.
func (c *clusterNetworks) Create(clusterNetwork *network.ClusterNetwork) (result *network.ClusterNetwork, err error) {
	result = &network.ClusterNetwork{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("clusternetworks").
		Body(clusterNetwork).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterNetwork and updates it. Returns the server's representation of the clusterNetwork, and an error, if there is any.
func (c *clusterNetworks) Update(clusterNetwork *network.ClusterNetwork) (result *network.ClusterNetwork, err error) {
	result = &network.ClusterNetwork{}
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
func (c *clusterNetworks) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clusternetworks").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterNetworks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the clusterNetwork, and returns the corresponding clusterNetwork object, and an error if there is any.
func (c *clusterNetworks) Get(name string, options v1.GetOptions) (result *network.ClusterNetwork, err error) {
	result = &network.ClusterNetwork{}
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
func (c *clusterNetworks) List(opts v1.ListOptions) (result *network.ClusterNetworkList, err error) {
	result = &network.ClusterNetworkList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterNetworks.
func (c *clusterNetworks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("clusternetworks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *clusterNetworks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.ClusterNetwork, err error) {
	result = &network.ClusterNetwork{}
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
