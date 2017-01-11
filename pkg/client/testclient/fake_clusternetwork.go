package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeClusterNetwork implements ClusterNetworkInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterNetwork struct {
	Fake *Fake
}

var clusterNetworksResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "clusternetworks"}

func (c *FakeClusterNetwork) Get(name string) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(clusterNetworksResource, name), &sdnapi.ClusterNetwork{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Create(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Update(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(clusterNetworksResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}
