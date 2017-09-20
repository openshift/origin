package internalversion

import (
	network "github.com/openshift/origin/pkg/network/apis/network"
	scheme "github.com/openshift/origin/pkg/network/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// HostSubnetsGetter has a method to return a HostSubnetInterface.
// A group's client should implement this interface.
type HostSubnetsGetter interface {
	HostSubnets() HostSubnetInterface
}

// HostSubnetInterface has methods to work with HostSubnet resources.
type HostSubnetInterface interface {
	Create(*network.HostSubnet) (*network.HostSubnet, error)
	Update(*network.HostSubnet) (*network.HostSubnet, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*network.HostSubnet, error)
	List(opts v1.ListOptions) (*network.HostSubnetList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.HostSubnet, err error)
	HostSubnetExpansion
}

// hostSubnets implements HostSubnetInterface
type hostSubnets struct {
	client rest.Interface
}

// newHostSubnets returns a HostSubnets
func newHostSubnets(c *NetworkClient) *hostSubnets {
	return &hostSubnets{
		client: c.RESTClient(),
	}
}

// Get takes name of the hostSubnet, and returns the corresponding hostSubnet object, and an error if there is any.
func (c *hostSubnets) Get(name string, options v1.GetOptions) (result *network.HostSubnet, err error) {
	result = &network.HostSubnet{}
	err = c.client.Get().
		Resource("hostsubnets").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of HostSubnets that match those selectors.
func (c *hostSubnets) List(opts v1.ListOptions) (result *network.HostSubnetList, err error) {
	result = &network.HostSubnetList{}
	err = c.client.Get().
		Resource("hostsubnets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested hostSubnets.
func (c *hostSubnets) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("hostsubnets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a hostSubnet and creates it.  Returns the server's representation of the hostSubnet, and an error, if there is any.
func (c *hostSubnets) Create(hostSubnet *network.HostSubnet) (result *network.HostSubnet, err error) {
	result = &network.HostSubnet{}
	err = c.client.Post().
		Resource("hostsubnets").
		Body(hostSubnet).
		Do().
		Into(result)
	return
}

// Update takes the representation of a hostSubnet and updates it. Returns the server's representation of the hostSubnet, and an error, if there is any.
func (c *hostSubnets) Update(hostSubnet *network.HostSubnet) (result *network.HostSubnet, err error) {
	result = &network.HostSubnet{}
	err = c.client.Put().
		Resource("hostsubnets").
		Name(hostSubnet.Name).
		Body(hostSubnet).
		Do().
		Into(result)
	return
}

// Delete takes name of the hostSubnet and deletes it. Returns an error if one occurs.
func (c *hostSubnets) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("hostsubnets").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *hostSubnets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("hostsubnets").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched hostSubnet.
func (c *hostSubnets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.HostSubnet, err error) {
	result = &network.HostSubnet{}
	err = c.client.Patch(pt).
		Resource("hostsubnets").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
