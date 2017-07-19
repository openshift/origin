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

// FakeClusterRoles implements ClusterRoleInterface
type FakeClusterRoles struct {
	Fake *FakeAuthorizationV1
}

var clusterrolesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterroles"}

var clusterrolesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "ClusterRole"}

func (c *FakeClusterRoles) Create(clusterRole *v1.ClusterRole) (result *v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterrolesResource, clusterRole), &v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRole), err
}

func (c *FakeClusterRoles) Update(clusterRole *v1.ClusterRole) (result *v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterrolesResource, clusterRole), &v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRole), err
}

func (c *FakeClusterRoles) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterrolesResource, name), &v1.ClusterRole{})
	return err
}

func (c *FakeClusterRoles) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterrolesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterRoleList{})
	return err
}

func (c *FakeClusterRoles) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterrolesResource, name), &v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRole), err
}

func (c *FakeClusterRoles) List(opts meta_v1.ListOptions) (result *v1.ClusterRoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterrolesResource, clusterrolesKind, opts), &v1.ClusterRoleList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterRoleList{}
	for _, item := range obj.(*v1.ClusterRoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterRoles.
func (c *FakeClusterRoles) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterrolesResource, opts))
}

// Patch applies the patch and returns the patched clusterRole.
func (c *FakeClusterRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolesResource, name, data, subresources...), &v1.ClusterRole{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterRole), err
}
