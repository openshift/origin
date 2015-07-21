package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeHostSubnet implements HostSubnetInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeHostSubnet struct {
	Fake *Fake
}

func (c *FakeHostSubnet) List() (*sdnapi.HostSubnetList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-subnets"}, &sdnapi.HostSubnetList{})
	return obj.(*sdnapi.HostSubnetList), err
}

func (c *FakeHostSubnet) Get(name string) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-subnets"}, &sdnapi.HostSubnet{})
	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Create(sdn *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-subnet"}, &sdnapi.HostSubnet{})
	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-subnet"})
	return nil
}

func (c *FakeHostSubnet) Watch(resourceVersion string) (watch.Interface, error) {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-subnets"})
	return nil, nil
}
