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

// FakeClusterPolicyBindings implements ClusterPolicyBindingInterface
type FakeClusterPolicyBindings struct {
	Fake *FakeAuthorizationV1
}

var clusterpolicybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterpolicybindings"}

var clusterpolicybindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "ClusterPolicyBinding"}

func (c *FakeClusterPolicyBindings) Create(clusterPolicyBinding *v1.ClusterPolicyBinding) (result *v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpolicybindingsResource, clusterPolicyBinding), &v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Update(clusterPolicyBinding *v1.ClusterPolicyBinding) (result *v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpolicybindingsResource, clusterPolicyBinding), &v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpolicybindingsResource, name), &v1.ClusterPolicyBinding{})
	return err
}

func (c *FakeClusterPolicyBindings) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpolicybindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterPolicyBindingList{})
	return err
}

func (c *FakeClusterPolicyBindings) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpolicybindingsResource, name), &v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicyBinding), err
}

func (c *FakeClusterPolicyBindings) List(opts meta_v1.ListOptions) (result *v1.ClusterPolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpolicybindingsResource, clusterpolicybindingsKind, opts), &v1.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterPolicyBindingList{}
	for _, item := range obj.(*v1.ClusterPolicyBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPolicyBindings.
func (c *FakeClusterPolicyBindings) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpolicybindingsResource, opts))
}

// Patch applies the patch and returns the patched clusterPolicyBinding.
func (c *FakeClusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpolicybindingsResource, name, data, subresources...), &v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPolicyBinding), err
}
