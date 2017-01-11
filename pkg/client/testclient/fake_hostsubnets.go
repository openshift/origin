package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeHostSubnet implements HostSubnetInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeHostSubnet struct {
	Fake *Fake
}

var hostSubnetsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "hostsubnets"}

func (c *FakeHostSubnet) Get(name string) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(hostSubnetsResource, name), &sdnapi.HostSubnet{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) List(opts kapi.ListOptions) (*sdnapi.HostSubnetList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(hostSubnetsResource, opts), &sdnapi.HostSubnetList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnetList), err
}

func (c *FakeHostSubnet) Create(inObj *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(hostSubnetsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Update(inObj *sdnapi.HostSubnet) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(hostSubnetsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(hostSubnetsResource, name), &sdnapi.HostSubnet{})
	return err
}

func (c *FakeHostSubnet) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(hostSubnetsResource, opts))
}
