package testclient

import (
	"k8s.io/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeNetNamespace implements NetNamespaceInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeNetNamespace struct {
	Fake *Fake
}

func (c *FakeNetNamespace) List() (*sdnapi.NetNamespaceList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-netnamespaces"}, &sdnapi.NetNamespaceList{})
	return obj.(*sdnapi.NetNamespaceList), err
}

func (c *FakeNetNamespace) Get(name string) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-netnamespaces"}, &sdnapi.NetNamespace{})
	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) Create(sdn *sdnapi.NetNamespace) (*sdnapi.NetNamespace, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-netnamespace"}, &sdnapi.NetNamespace{})
	return obj.(*sdnapi.NetNamespace), err
}

func (c *FakeNetNamespace) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-netnamespace"})
	return nil
}

func (c *FakeNetNamespace) Watch(resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-netnamespaces"})
	return nil, nil
}
