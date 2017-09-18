package client

import (
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterNetworkingInterface has methods to work with ClusterNetwork resources
type ClusterNetworkingInterface interface {
	ClusterNetwork() ClusterNetworkInterface
}

// ClusterNetworkInterface exposes methods on clusterNetwork resources.
type ClusterNetworkInterface interface {
	Get(name string, options metav1.GetOptions) (*networkapi.ClusterNetwork, error)
	Create(sub *networkapi.ClusterNetwork) (*networkapi.ClusterNetwork, error)
	Update(sub *networkapi.ClusterNetwork) (*networkapi.ClusterNetwork, error)
}

// clusterNetwork implements ClusterNetworkInterface interface
type clusterNetwork struct {
	r *Client
}

// newClusterNetwork returns a clusterNetwork
func newClusterNetwork(c *Client) *clusterNetwork {
	return &clusterNetwork{
		r: c,
	}
}

// Get returns information about a particular network
func (c *clusterNetwork) Get(networkName string, options metav1.GetOptions) (result *networkapi.ClusterNetwork, err error) {
	result = &networkapi.ClusterNetwork{}
	err = c.r.Get().Resource("clusterNetworks").Name(networkName).Do().Into(result)
	return
}

// Create creates a new ClusterNetwork. Returns the server's representation of ClusterNetwork and error if one occurs.
func (c *clusterNetwork) Create(cn *networkapi.ClusterNetwork) (result *networkapi.ClusterNetwork, err error) {
	result = &networkapi.ClusterNetwork{}
	err = c.r.Post().Resource("clusterNetworks").Body(cn).Do().Into(result)
	return
}

// Update updates the ClusterNetwork on the server. Returns the server's representation of the ClusterNetwork and error if one occurs.
func (c *clusterNetwork) Update(cn *networkapi.ClusterNetwork) (result *networkapi.ClusterNetwork, err error) {
	result = &networkapi.ClusterNetwork{}
	err = c.r.Put().Resource("clusterNetworks").Name(cn.Name).Body(cn).Do().Into(result)
	return
}
