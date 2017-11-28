package fake

import (
	authorization_v1 "github.com/openshift/api/authorization/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Get takes name of the clusterPolicyBinding, and returns the corresponding clusterPolicyBinding object, and an error if there is any.
func (c *FakeClusterPolicyBindings) Get(name string, options v1.GetOptions) (result *authorization_v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpolicybindingsResource, name), &authorization_v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterPolicyBinding), err
}

// List takes label and field selectors, and returns the list of ClusterPolicyBindings that match those selectors.
func (c *FakeClusterPolicyBindings) List(opts v1.ListOptions) (result *authorization_v1.ClusterPolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpolicybindingsResource, clusterpolicybindingsKind, opts), &authorization_v1.ClusterPolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.ClusterPolicyBindingList{}
	for _, item := range obj.(*authorization_v1.ClusterPolicyBindingList).Items {
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

// Create takes the representation of a clusterPolicyBinding and creates it.  Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *FakeClusterPolicyBindings) Create(clusterPolicyBinding *authorization_v1.ClusterPolicyBinding) (result *authorization_v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpolicybindingsResource, clusterPolicyBinding), &authorization_v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterPolicyBinding), err
}

// Update takes the representation of a clusterPolicyBinding and updates it. Returns the server's representation of the clusterPolicyBinding, and an error, if there is any.
func (c *FakeClusterPolicyBindings) Update(clusterPolicyBinding *authorization_v1.ClusterPolicyBinding) (result *authorization_v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpolicybindingsResource, clusterPolicyBinding), &authorization_v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterPolicyBinding), err
}

// Delete takes name of the clusterPolicyBinding and deletes it. Returns an error if one occurs.
func (c *FakeClusterPolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpolicybindingsResource, name), &authorization_v1.ClusterPolicyBinding{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterPolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpolicybindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.ClusterPolicyBindingList{})
	return err
}

// Patch applies the patch and returns the patched clusterPolicyBinding.
func (c *FakeClusterPolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.ClusterPolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpolicybindingsResource, name, data, subresources...), &authorization_v1.ClusterPolicyBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterPolicyBinding), err
}
