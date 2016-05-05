package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeHostSubnet implements HostSubnetInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeHostSubnet struct {
	Fake *Fake
}

func (c *FakeHostSubnet) Get(name string) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("hostsubnets", name), &sdnapi.HostSubnet{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) List(opts kapi.ListOptions) (*sdnapi.HostSubnetList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("hostsubnets", opts), &sdnapi.HostSubnetList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnetList), err
}

func (c *FakeHostSubnet) Create(inObj *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("hostsubnets", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Update(inObj *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("hostsubnets", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("hostsubnets", name), &sdnapi.HostSubnet{})
	return err
}

func (c *FakeHostSubnet) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("hostsubnets", opts))
}
