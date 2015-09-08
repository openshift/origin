package client

import (
	"k8s.io/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// NetNamespaceInterface has methods to work with NetNamespace resources
type NetNamespacesInterface interface {
	NetNamespaces() NetNamespaceInterface
}

// NetNamespaceInterface exposes methods on NetNamespace resources.
type NetNamespaceInterface interface {
	List() (*sdnapi.NetNamespaceList, error)
	Get(name string) (*sdnapi.NetNamespace, error)
	Create(sub *sdnapi.NetNamespace) (*sdnapi.NetNamespace, error)
	Delete(name string) error
	Watch(resourceVersion string) (watch.Interface, error)
}

// netNamespace implements NetNamespaceInterface interface
type netNamespace struct {
	r *Client
}

// newNetNamespace returns a NetNamespace
func newNetNamespace(c *Client) *netNamespace {
	return &netNamespace{
		r: c,
	}
}

// List returns a list of NetNamespaces that match the label and field selectors.
func (c *netNamespace) List() (result *sdnapi.NetNamespaceList, err error) {
	result = &sdnapi.NetNamespaceList{}
	err = c.r.Get().
		Resource("netNamespaces").
		Do().
		Into(result)
	return
}

// Get returns information about a particular NetNamespace or an error
func (c *netNamespace) Get(netname string) (result *sdnapi.NetNamespace, err error) {
	result = &sdnapi.NetNamespace{}
	err = c.r.Get().Resource("netNamespaces").Name(netname).Do().Into(result)
	return
}

// Create creates a new NetNamespace. Returns the server's representation of the NetNamespace and error if one occurs.
func (c *netNamespace) Create(netNamespace *sdnapi.NetNamespace) (result *sdnapi.NetNamespace, err error) {
	result = &sdnapi.NetNamespace{}
	err = c.r.Post().Resource("netNamespaces").Body(netNamespace).Do().Into(result)
	return
}

// Delete takes the name of the NetNamespace, and returns an error if one occurs during deletion of the NetNamespace
func (c *netNamespace) Delete(name string) error {
	return c.r.Delete().Resource("netNamespaces").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested NetNamespaces
func (c *netNamespace) Watch(resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("netNamespaces").
		Param("resourceVersion", resourceVersion).
		Watch()
}
