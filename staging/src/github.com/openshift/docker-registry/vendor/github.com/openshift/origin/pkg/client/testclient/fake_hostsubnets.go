package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

// FakeHostSubnet implements HostSubnetInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeHostSubnet struct {
	Fake *Fake
}

var hostSubnetsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "hostsubnets"}
var hostSubnetsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "HostSubnet"}

func (c *FakeHostSubnet) Get(name string, options metav1.GetOptions) (*networkapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(hostSubnetsResource, name), &networkapi.HostSubnet{})
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.HostSubnet), err
}

func (c *FakeHostSubnet) List(opts metav1.ListOptions) (*networkapi.HostSubnetList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(hostSubnetsResource, hostSubnetsKind, opts), &networkapi.HostSubnetList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.HostSubnetList), err
}

func (c *FakeHostSubnet) Create(inObj *networkapi.HostSubnet) (*networkapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(hostSubnetsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.HostSubnet), err
}

func (c *FakeHostSubnet) Update(inObj *networkapi.HostSubnet) (*networkapi.HostSubnet, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(hostSubnetsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*networkapi.HostSubnet), err
}

func (c *FakeHostSubnet) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(hostSubnetsResource, name), &networkapi.HostSubnet{})
	return err
}

func (c *FakeHostSubnet) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(hostSubnetsResource, opts))
}
