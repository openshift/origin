package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterPolicies implements ClusterPolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterPolicies struct {
	Fake *Fake
}

var clusterPoliciesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterpolicies"}

func (c *FakeClusterPolicies) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(clusterPoliciesResource, name), &authorizationapi.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicy), err
}

func (c *FakeClusterPolicies) List(opts metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewRootListAction(clusterPoliciesResource, optsv1), &authorizationapi.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyList), err
}

func (c *FakeClusterPolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(clusterPoliciesResource, name), &authorizationapi.ClusterPolicy{})
	return err
}

func (c *FakeClusterPolicies) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(core.NewRootWatchAction(clusterPoliciesResource, optsv1))
}
