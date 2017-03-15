package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeHostSubnet implements HostSubnetInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeHostSubnet struct {
	Fake *Fake
}

var hostSubnetsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "hostsubnets"}

func (c *FakeHostSubnet) Get(name string) (*sdnapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(hostSubnetsResource, name), &sdnapi.HostSubnet{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.HostSubnet), err
}

func (c *FakeHostSubnet) List(opts metainternal.ListOptions) (*sdnapi.HostSubnetList, error) {
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

func (c *FakeHostSubnet) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(hostSubnetsResource, opts))
}
