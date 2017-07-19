package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterPolicies implements ClusterPolicyInterface
type FakeClusterPolicies struct {
	Fake *FakeAuthorizationV1
}

var clusterpoliciesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterpolicies"}

var clusterpoliciesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "ClusterPolicy"}

func (c *FakeClusterPolicies) Create(clusterPolicy *v1.ClusterPolicy) (result *v1.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpoliciesResource, clusterPolicy), &v1.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicy), err
}

func (c *FakeClusterPolicies) Update(clusterPolicy *v1.ClusterPolicy) (result *v1.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpoliciesResource, clusterPolicy), &v1.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicy), err
}

func (c *FakeClusterPolicies) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpoliciesResource, name), &v1.ClusterPolicy{})
	return err
}

func (c *FakeClusterPolicies) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpoliciesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterPolicyList{})
	return err
}

func (c *FakeClusterPolicies) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpoliciesResource, name), &v1.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicy), err
}

func (c *FakeClusterPolicies) List(opts meta_v1.ListOptions) (result *v1.ClusterPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpoliciesResource, clusterpoliciesKind, opts), &v1.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterPolicyList{}
	for _, item := range obj.(*v1.ClusterPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPolicies.
func (c *FakeClusterPolicies) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpoliciesResource, opts))
}

// Patch applies the patch and returns the patched clusterPolicy.
func (c *FakeClusterPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpoliciesResource, name, data, subresources...), &v1.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicy), err
}
