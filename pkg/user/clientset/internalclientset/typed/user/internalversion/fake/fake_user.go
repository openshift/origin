package fake

import (
	api "github.com/openshift/origin/pkg/user/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeUsers implements UserResourceInterface
type FakeUsers struct {
	Fake *FakeUser
	ns   string
}

var usersResource = schema.GroupVersionResource{Group: "user.openshift.io", Version: "", Resource: "users"}

func (c *FakeUsers) Create(user *api.User) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(usersResource, c.ns, user), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) Update(user *api.User) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(usersResource, c.ns, user), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(usersResource, c.ns, name), &api.User{})

	return err
}

func (c *FakeUsers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(usersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.UserList{})
	return err
}

func (c *FakeUsers) Get(name string, options v1.GetOptions) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(usersResource, c.ns, name), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) List(opts v1.ListOptions) (result *api.UserList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(usersResource, c.ns, opts), &api.UserList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.UserList{}
	for _, item := range obj.(*api.UserList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested users.
func (c *FakeUsers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(usersResource, c.ns, opts))

}

// Patch applies the patch and returns the patched user.
func (c *FakeUsers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(usersResource, c.ns, name, data, subresources...), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}
