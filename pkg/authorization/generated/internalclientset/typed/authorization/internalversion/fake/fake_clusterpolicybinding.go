package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
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

var clusterpolicybindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "ClusterPolicyBinding"}

func (c *FakeClusterPolicyBindings) Create(clusterPolicyBinding *authorization.ClusterPolicyBinding) (result *authorization.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpolicybindingsResource, clusterPolicyBinding), &authorization.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Update(clusterPolicyBinding *authorization.ClusterPolicyBinding) (result *authorization.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpolicybindingsResource, clusterPolicyBinding), &authorization.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpolicybindingsResource, name), &authorization.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpolicybindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &authorization.ClusterPolicyBindingList{})
	return err
}

func (c *FakeClusterPolicyBindings) Get(name string, options v1.GetOptions) (result *authorization.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpolicybindingsResource, name), &authorization.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(opts v1.ListOptions) (result *authorization.ClusterPolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpolicybindingsResource, clusterpolicybindingsKind, opts), &authorization.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization.ClusterPolicyBindingList{}
	for _, item := range obj.(*authorization.ClusterPolicyBindingList).Items {
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
func (c *FakeClusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpolicybindingsResource, name, data, subresources...), &authorization.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicyBinding), err
}
