package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeClusterPolicies struct {
	Fake *Fake
}

func (c *FakeClusterPolicies) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-clusterPolicies"}, &authorizationapi.ClusterPolicyList{})
	return obj.(*authorizationapi.ClusterPolicyList), err
}

func (c *FakeClusterPolicies) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-clusterPolicy"}, &authorizationapi.ClusterPolicy{})
	return obj.(*authorizationapi.ClusterPolicy), err
}

func (c *FakeClusterPolicies) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-clusterPolicy", Value: name})
	return nil
}
