package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeClusterNetwork implements ClusterNetworkInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterNetwork struct {
	Fake *Fake
}

func (c *FakeClusterNetwork) Get(name string) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusternetworks", name), &sdnapi.ClusterNetwork{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Create(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("clusternetworks", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Update(inObj *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("clusternetworks", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.ClusterNetwork), err
}
