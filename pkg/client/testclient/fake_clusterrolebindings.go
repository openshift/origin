package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterRoleBindings implements ClusterRoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterRoleBindings struct {
	Fake *Fake
}

func (c *FakeClusterRoleBindings) Get(name string) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusterrolebindings", name), &authorizationapi.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleBindingList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("clusterrolebindings", label, field), &authorizationapi.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBindingList), err
}

func (c *FakeClusterRoleBindings) Create(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("clusterrolebindings", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("clusterrolebindings", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("clusterrolebindings", name), &authorizationapi.ClusterRoleBinding{})
	return err
}
