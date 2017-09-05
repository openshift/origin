package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

// FakeClusterNetwork implements ClusterNetworkInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterNetwork struct {
	Fake *Fake
}

var clusterNetworksResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusternetworks"}

func (c *FakeClusterNetwork) Get(name string, options metav1.GetOptions) (*networkapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterNetworksResource, name), &networkapi.ClusterNetwork{})
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Create(inObj *networkapi.ClusterNetwork) (*networkapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Update(inObj *networkapi.ClusterNetwork) (*networkapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.ClusterNetwork), err
}
