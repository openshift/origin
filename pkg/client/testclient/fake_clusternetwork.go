package testclient

import (
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeClusterNetwork implements ClusterNetworkInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterNetwork struct {
	Fake *Fake
}

func (c *FakeClusterNetwork) Get(name string) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-network"}, &sdnapi.ClusterNetwork{})
	return obj.(*sdnapi.ClusterNetwork), err
}

func (c *FakeClusterNetwork) Create(sdn *sdnapi.ClusterNetwork) (*sdnapi.ClusterNetwork, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-network"}, &sdnapi.ClusterNetwork{})
	return obj.(*sdnapi.ClusterNetwork), err
}
