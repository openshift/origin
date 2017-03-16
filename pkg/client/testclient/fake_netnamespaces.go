package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeNetNamespace implements NetNamespaceInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeNetNamespace struct {
	Fake *Fake
}

var netNamespacesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "netnamespaces"}

func (c *FakeNetNamespace) Get(name string, options metav1.GetOptions) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(netNamespacesResource, name), &sdnapi.NetNamespace{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) List(opts metainternal.ListOptions) (*sdnapi.NetNamespaceList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(netNamespacesResource, optsv1), &sdnapi.NetNamespaceList{})
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

func (c *FakeNetNamespace) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(netNamespacesResource, optsv1))
}
