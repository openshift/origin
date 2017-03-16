package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicyBindings struct {
	Fake *Fake
}

var clusterPolicyBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterpolicybindings"}

func (c *FakeClusterPolicyBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(clusterPolicyBindingsResource, name), &authorizationapi.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(opts metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewRootListAction(clusterPolicyBindingsResource, optsv1), &authorizationapi.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBindingList), err
}

func (c *FakeClusterPolicyBindings) Create(inObj *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(clusterPolicyBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(clusterPolicyBindingsResource, name), &authorizationapi.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(core.NewRootWatchAction(clusterPolicyBindingsResource, optsv1))
}
