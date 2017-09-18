package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakeClusterPolicies implements ClusterPolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicies struct {
	Fake *Fake
}

var clusterPoliciesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterpolicies"}
var clusterPoliciesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "ClusterPolicy"}

func (c *FakeClusterPolicies) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterPoliciesResource, name), &authorizationapi.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicy), err
}

func (c *FakeClusterPolicies) List(opts metav1.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(clusterPoliciesResource, clusterPoliciesKind, opts), &authorizationapi.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyList), err
}

func (c *FakeClusterPolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(clusterPoliciesResource, name), &authorizationapi.ClusterPolicy{})
	return err
}

func (c *FakeClusterPolicies) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(clusterPoliciesResource, opts))
}
