package internalversion

import (
	network "github.com/openshift/origin/pkg/network/apis/network"
	scheme "github.com/openshift/origin/pkg/network/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// NetNamespacesGetter has a method to return a NetNamespaceInterface.
// A group's client should implement this interface.
type NetNamespacesGetter interface {
	NetNamespaces() NetNamespaceInterface
}

// NetNamespaceInterface has methods to work with NetNamespace resources.
type NetNamespaceInterface interface {
	Create(*network.NetNamespace) (*network.NetNamespace, error)
	Update(*network.NetNamespace) (*network.NetNamespace, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*network.NetNamespace, error)
	List(opts v1.ListOptions) (*network.NetNamespaceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.NetNamespace, err error)
	NetNamespaceExpansion
}

// netNamespaces implements NetNamespaceInterface
type netNamespaces struct {
	client rest.Interface
}

// newNetNamespaces returns a NetNamespaces
func newNetNamespaces(c *NetworkClient) *netNamespaces {
	return &netNamespaces{
		client: c.RESTClient(),
	}
}

// Get takes name of the netNamespace, and returns the corresponding netNamespace object, and an error if there is any.
func (c *netNamespaces) Get(name string, options v1.GetOptions) (result *network.NetNamespace, err error) {
	result = &network.NetNamespace{}
	err = c.client.Get().
		Resource("netnamespaces").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of NetNamespaces that match those selectors.
func (c *netNamespaces) List(opts v1.ListOptions) (result *network.NetNamespaceList, err error) {
	result = &network.NetNamespaceList{}
	err = c.client.Get().
		Resource("netnamespaces").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested netNamespaces.
func (c *netNamespaces) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("netnamespaces").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a netNamespace and creates it.  Returns the server's representation of the netNamespace, and an error, if there is any.
func (c *netNamespaces) Create(netNamespace *network.NetNamespace) (result *network.NetNamespace, err error) {
	result = &network.NetNamespace{}
	err = c.client.Post().
		Resource("netnamespaces").
		Body(netNamespace).
		Do().
		Into(result)
	return
}

// Update takes the representation of a netNamespace and updates it. Returns the server's representation of the netNamespace, and an error, if there is any.
func (c *netNamespaces) Update(netNamespace *network.NetNamespace) (result *network.NetNamespace, err error) {
	result = &network.NetNamespace{}
	err = c.client.Put().
		Resource("netnamespaces").
		Name(netNamespace.Name).
		Body(netNamespace).
		Do().
		Into(result)
	return
}

// Delete takes name of the netNamespace and deletes it. Returns an error if one occurs.
func (c *netNamespaces) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("netnamespaces").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *netNamespaces) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("netnamespaces").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched netNamespace.
func (c *netNamespaces) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.NetNamespace, err error) {
	result = &network.NetNamespace{}
	err = c.client.Patch(pt).
		Resource("netnamespaces").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
