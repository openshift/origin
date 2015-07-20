package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicyBindings struct {
	Fake *Fake
}

func (c *FakeClusterPolicyBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-clusterPolicyBindings"}, &authorizationapi.ClusterPolicyBindingList{})
	return obj.(*authorizationapi.ClusterPolicyBindingList), err
}

func (c *FakeClusterPolicyBindings) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-clusterPolicyBinding"}, &authorizationapi.ClusterPolicyBinding{})
	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Create(policyBinding *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-clusterPolicyBinding", Value: policyBinding}, &authorizationapi.ClusterPolicyBinding{})
	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-clusterPolicyBinding", Value: name})
	return nil
}

func (c *FakeClusterPolicyBindings) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-clusterPolicyBinding"})
	return nil, nil
}
