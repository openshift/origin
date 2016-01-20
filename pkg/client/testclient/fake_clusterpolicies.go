package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterPolicies implements ClusterPolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicies struct {
	Fake *Fake
}

func (c *FakeClusterPolicies) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusterpolicies", name), &authorizationapi.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicy), err
}

func (c *FakeClusterPolicies) List(opts kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("clusterpolicies", opts), &authorizationapi.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyList), err
}

func (c *FakeClusterPolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("clusterpolicies", name), &authorizationapi.ClusterPolicy{})
	return err
}

func (c *FakeClusterPolicies) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("clusterpolicies", opts))
}
