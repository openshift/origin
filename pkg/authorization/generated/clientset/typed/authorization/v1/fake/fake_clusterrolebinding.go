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

// FakeClusterRoleBindings implements ClusterRoleBindingInterface
type FakeClusterRoleBindings struct {
	Fake *FakeAuthorizationV1
}

var clusterrolebindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterrolebindings"}

var clusterrolebindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "ClusterRoleBinding"}

func (c *FakeClusterRoleBindings) Create(clusterRoleBinding *v1.ClusterRoleBinding) (result *v1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterrolebindingsResource, clusterRoleBinding), &v1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(clusterRoleBinding *v1.ClusterRoleBinding) (result *v1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterrolebindingsResource, clusterRoleBinding), &v1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterrolebindingsResource, name), &v1.ClusterRoleBinding{})
	return err
}

func (c *FakeClusterRoleBindings) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterrolebindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterRoleBindingList{})
	return err
}

func (c *FakeClusterRoleBindings) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterrolebindingsResource, name), &v1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) List(opts meta_v1.ListOptions) (result *v1.ClusterRoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterrolebindingsResource, clusterrolebindingsKind, opts), &v1.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterRoleBindingList{}
	for _, item := range obj.(*v1.ClusterRoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterRoleBindings.
func (c *FakeClusterRoleBindings) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterrolebindingsResource, opts))
}

// Patch applies the patch and returns the patched clusterRoleBinding.
func (c *FakeClusterRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolebindingsResource, name, data, subresources...), &v1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRoleBinding), err
}
