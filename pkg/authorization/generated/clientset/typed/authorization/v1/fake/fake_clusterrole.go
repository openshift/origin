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

// FakeClusterRoles implements ClusterRoleInterface
type FakeClusterRoles struct {
	Fake *FakeAuthorizationV1
}

var clusterrolesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterroles"}

var clusterrolesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "ClusterRole"}

// Get takes name of the clusterRole, and returns the corresponding clusterRole object, and an error if there is any.
func (c *FakeClusterRoles) Get(name string, options v1.GetOptions) (result *authorization_v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterrolesResource, name), &authorization_v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterRole), err
}

// List takes label and field selectors, and returns the list of ClusterRoles that match those selectors.
func (c *FakeClusterRoles) List(opts v1.ListOptions) (result *authorization_v1.ClusterRoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterrolesResource, clusterrolesKind, opts), &authorization_v1.ClusterRoleList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.ClusterRoleList{}
	for _, item := range obj.(*authorization_v1.ClusterRoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterRoles.
func (c *FakeClusterRoles) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterrolesResource, opts))
}

// Create takes the representation of a clusterRole and creates it.  Returns the server's representation of the clusterRole, and an error, if there is any.
func (c *FakeClusterRoles) Create(clusterRole *authorization_v1.ClusterRole) (result *authorization_v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterrolesResource, clusterRole), &authorization_v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterRole), err
}

// Update takes the representation of a clusterRole and updates it. Returns the server's representation of the clusterRole, and an error, if there is any.
func (c *FakeClusterRoles) Update(clusterRole *authorization_v1.ClusterRole) (result *authorization_v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterrolesResource, clusterRole), &authorization_v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterRole), err
}

// Delete takes name of the clusterRole and deletes it. Returns an error if one occurs.
func (c *FakeClusterRoles) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterrolesResource, name), &authorization_v1.ClusterRole{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterRoles) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterrolesResource, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.ClusterRoleList{})
	return err
}

// Patch applies the patch and returns the patched clusterRole.
func (c *FakeClusterRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolesResource, name, data, subresources...), &authorization_v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.ClusterRole), err
}
