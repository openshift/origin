package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

// FakeNetNamespace implements NetNamespaceInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeNetNamespace struct {
	Fake *Fake
}

var netNamespacesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "netnamespaces"}
var netNamespacesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "NetNamespace"}

func (c *FakeNetNamespace) Get(name string, options metav1.GetOptions) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(netNamespacesResource, name), &sdnapi.NetNamespace{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) List(opts metav1.ListOptions) (*sdnapi.NetNamespaceList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(netNamespacesResource, netNamespacesKind, opts), &sdnapi.NetNamespaceList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.NetNamespaceList), err
}

func (c *FakeNetNamespace) Create(inObj *sdnapi.NetNamespace) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(netNamespacesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) Update(inObj *sdnapi.NetNamespace) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(netNamespacesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(netNamespacesResource, name), &sdnapi.NetNamespace{})
	return err
}

func (c *FakeNetNamespace) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(netNamespacesResource, opts))
}
