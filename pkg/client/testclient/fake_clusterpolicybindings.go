package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicyBindings struct {
	Fake *Fake
}

var clusterPolicyBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterpolicybindings"}
var clusterPolicyBindingsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "ClusterPolicyBinding"}

func (c *FakeClusterPolicyBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterPolicyBindingsResource, name), &authorizationapi.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(opts metav1.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(clusterPolicyBindingsResource, clusterPolicyBindingsKind, opts), &authorizationapi.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBindingList), err
}

func (c *FakeClusterPolicyBindings) Create(inObj *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusterPolicyBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(clusterPolicyBindingsResource, name), &authorizationapi.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(clusterPolicyBindingsResource, opts))
}
