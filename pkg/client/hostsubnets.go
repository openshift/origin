package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

// HostSubnetInterface has methods to work with HostSubnet resources
type HostSubnetsInterface interface {
	HostSubnets() HostSubnetInterface
}

// HostSubnetInterface exposes methods on HostSubnet resources.
type HostSubnetInterface interface {
	List(opts metav1.ListOptions) (*networkapi.HostSubnetList, error)
	Get(name string, options metav1.GetOptions) (*networkapi.HostSubnet, error)
	Create(sub *networkapi.HostSubnet) (*networkapi.HostSubnet, error)
	Update(sub *networkapi.HostSubnet) (*networkapi.HostSubnet, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

// hostSubnet implements HostSubnetInterface interface
type hostSubnet struct {
	r *Client
}

// newHostSubnet returns a hostsubnet
func newHostSubnet(c *Client) *hostSubnet {
	return &hostSubnet{
		r: c,
	}
}

// List returns a list of hostsubnets that match the label and field selectors.
func (c *hostSubnet) List(opts metav1.ListOptions) (result *networkapi.HostSubnetList, err error) {
	result = &networkapi.HostSubnetList{}
	err = c.r.Get().
		Resource("hostSubnets").
		VersionedParams(&opts, kapi.ParameterCodec).
		Do().
		Into(result)
	return
}

// Get returns host subnet information for a given host or an error
func (c *hostSubnet) Get(hostName string, options metav1.GetOptions) (result *networkapi.HostSubnet, err error) {
	result = &networkapi.HostSubnet{}
	err = c.r.Get().Resource("hostSubnets").Name(hostName).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Create creates a new host subnet. Returns the server's representation of the host subnet and error if one occurs.
func (c *hostSubnet) Create(hostSubnet *networkapi.HostSubnet) (result *networkapi.HostSubnet, err error) {
	result = &networkapi.HostSubnet{}
	err = c.r.Post().Resource("hostSubnets").Body(hostSubnet).Do().Into(result)
	return
}

// Update updates existing host subnet. Returns the server's representation of the host subnet and error if one occurs.
func (c *hostSubnet) Update(hostSubnet *networkapi.HostSubnet) (result *networkapi.HostSubnet, err error) {
	result = &networkapi.HostSubnet{}
	err = c.r.Put().Resource("hostSubnets").Name(hostSubnet.Name).Body(hostSubnet).Do().Into(result)
	return
}

// Delete takes the name of the host, and returns an error if one occurs during deletion of the subnet
func (c *hostSubnet) Delete(name string) error {
	return c.r.Delete().Resource("hostSubnets").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested subnets
func (c *hostSubnet) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("hostSubnets").
		VersionedParams(&opts, kapi.ParameterCodec).
		Watch()
}
