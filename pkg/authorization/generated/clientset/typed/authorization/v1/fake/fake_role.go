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

// FakeRoles implements RoleInterface
type FakeRoles struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var rolesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "roles"}

var rolesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "Role"}

func (c *FakeRoles) Create(role *v1.Role) (result *v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolesResource, c.ns, role), &v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Role), err
}

func (c *FakeRoles) Update(role *v1.Role) (result *v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolesResource, c.ns, role), &v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Role), err
}

func (c *FakeRoles) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolesResource, c.ns, name), &v1.Role{})

	return err
}

func (c *FakeRoles) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.RoleList{})
	return err
}

func (c *FakeRoles) Get(name string, options meta_v1.GetOptions) (result *v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolesResource, c.ns, name), &v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Role), err
}

func (c *FakeRoles) List(opts meta_v1.ListOptions) (result *v1.RoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolesResource, rolesKind, c.ns, opts), &v1.RoleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RoleList{}
	for _, item := range obj.(*v1.RoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roles.
func (c *FakeRoles) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched role.
func (c *FakeRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolesResource, c.ns, name, data, subresources...), &v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Role), err
}
