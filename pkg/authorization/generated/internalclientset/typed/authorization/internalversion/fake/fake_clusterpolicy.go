package fake

import (
	api "github.com/openshift/origin/pkg/authorization/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterPolicies implements ClusterPolicyInterface
type FakeClusterPolicies struct {
	Fake *FakeAuthorization
}

var clusterpoliciesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "clusterpolicies"}

func (c *FakeClusterPolicies) Create(clusterPolicy *api.ClusterPolicy) (result *api.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpoliciesResource, clusterPolicy), &api.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicy), err
}

func (c *FakeClusterPolicies) Update(clusterPolicy *api.ClusterPolicy) (result *api.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpoliciesResource, clusterPolicy), &api.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicy), err
}

func (c *FakeClusterPolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpoliciesResource, name), &api.ClusterPolicy{})
	return err
}

func (c *FakeClusterPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpoliciesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterPolicyList{})
	return err
}

func (c *FakeClusterPolicies) Get(name string, options v1.GetOptions) (result *api.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpoliciesResource, name), &api.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicy), err
}

func (c *FakeClusterPolicies) List(opts v1.ListOptions) (result *api.ClusterPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpoliciesResource, opts), &api.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ClusterPolicyList{}
	for _, item := range obj.(*api.ClusterPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPolicies.
func (c *FakeClusterPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpoliciesResource, opts))
}

// Patch applies the patch and returns the patched clusterPolicy.
func (c *FakeClusterPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpoliciesResource, name, data, subresources...), &api.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicy), err
}
