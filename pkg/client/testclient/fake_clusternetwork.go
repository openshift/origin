package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

// FakeClusterNetwork implements ClusterNetworkInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterNetwork struct {
	Fake *Fake
}

var clusterNetworksResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusternetworks"}

func (c *FakeClusterNetwork) Get(name string, options metav1.GetOptions) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterNetworksResource, name), &sdnapi.ClusterNetwork{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Create(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Update(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}
