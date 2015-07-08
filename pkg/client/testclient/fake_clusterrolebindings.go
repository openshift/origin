package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterRoleBindings implements ClusterRoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterRoleBindings struct {
	Fake *Fake
}

func (c *FakeClusterRoleBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleBindingList, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "list-clusterRoleBindings"}, &authorizationapi.ClusterRoleBindingList{})
	return obj.(*authorizationapi.ClusterRoleBindingList), err
}

func (c *FakeClusterRoleBindings) Get(name string) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "get-clusterRoleBinding"}, &authorizationapi.ClusterRoleBinding{})
	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Create(roleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-clusterRoleBinding", Value: roleBinding}, &authorizationapi.ClusterRoleBinding{})
	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(roleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "update-clusterRoleBinding"}, &authorizationapi.ClusterRoleBinding{})
	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, ktestclient.FakeAction{Action: "delete-clusterRoleBinding", Value: name})
	return nil
}
