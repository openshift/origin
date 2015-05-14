package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	_ "github.com/openshift/origin/pkg/sdn/api/v1beta1"
	_ "github.com/openshift/origin/pkg/sdn/api/v1beta3"
)

// HostSubnetInterface has methods to work with HostSubnet resources
type HostSubnetsInterface interface {
	HostSubnets() HostSubnetInterface
}

// HostSubnetInterface exposes methods on HostSubnet resources.
type HostSubnetInterface interface {
	List() (*sdnapi.HostSubnetList, error)
	Get(name string) (*sdnapi.HostSubnet, error)
	Create(sub *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error)
	Delete(name string) error
	Watch(resourceVersion string) (watch.Interface, error)
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
func (c *hostSubnet) List() (result *sdnapi.HostSubnetList, err error) {
	result = &sdnapi.HostSubnetList{}
	err = c.r.Get().
		Resource("hostSubnet").
		Do().
		Into(result)
	return
}

// Get returns information about a particular user or an error
func (c *hostSubnet) Get(hostName string) (result *sdnapi.HostSubnet, err error) {
	result = &sdnapi.HostSubnet{}
	err = c.r.Get().Resource("hostSubnet").Name(hostName).Do().Into(result)
	return
}

// Create creates a new user. Returns the server's representation of the user and error if one occurs.
func (c *hostSubnet) Create(hostSubnet *sdnapi.HostSubnet) (result *sdnapi.HostSubnet, err error) {
	result = &sdnapi.HostSubnet{}
	err = c.r.Post().Resource("hostSubnet").Body(hostSubnet).Do().Into(result)
	return
}

// Delete takes the name of the host, and returns an error if one occurs during deletion of the subnet
func (c *hostSubnet) Delete(name string) error {
	return c.r.Delete().Resource("hostSubnet").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested subnets
func (c *hostSubnet) Watch(resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("hostSubnet").
		Param("resourceVersion", resourceVersion).
		Watch()
}
