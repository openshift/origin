package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicyBindings struct {
	Fake *Fake
}

func (c *FakeClusterPolicyBindings) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusterpolicybindings", name), &authorizationapi.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("clusterpolicybindings", label, field), &authorizationapi.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBindingList), err
}

func (c *FakeClusterPolicyBindings) Create(inObj *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("clusterpolicybindings", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("clusterpolicybindings", name), &authorizationapi.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("clusterpolicybindings", label, field, resourceVersion))
}
