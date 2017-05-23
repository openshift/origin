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

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface
type FakeClusterPolicyBindings struct {
	Fake *FakeAuthorization
}

var clusterpolicybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "clusterpolicybindings"}

func (c *FakeClusterPolicyBindings) Create(clusterPolicyBinding *api.ClusterPolicyBinding) (result *api.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpolicybindingsResource, clusterPolicyBinding), &api.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Update(clusterPolicyBinding *api.ClusterPolicyBinding) (result *api.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpolicybindingsResource, clusterPolicyBinding), &api.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpolicybindingsResource, name), &api.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpolicybindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterPolicyBindingList{})
	return err
}

func (c *FakeClusterPolicyBindings) Get(name string, options v1.GetOptions) (result *api.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpolicybindingsResource, name), &api.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(opts v1.ListOptions) (result *api.ClusterPolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpolicybindingsResource, opts), &api.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ClusterPolicyBindingList{}
	for _, item := range obj.(*api.ClusterPolicyBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPolicyBindings.
func (c *FakeClusterPolicyBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpolicybindingsResource, opts))
}

// Patch applies the patch and returns the patched clusterPolicyBinding.
func (c *FakeClusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpolicybindingsResource, name, data, subresources...), &api.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterPolicyBinding), err
}
