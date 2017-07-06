package fake

import (
	user "github.com/openshift/origin/pkg/user/apis/user"
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

var usersKind = schema.GroupVersionKind{Group: "user.openshift.io", Version: "", Kind: "User"}

func (c *FakeUsers) Create(userResource *user.User) (result *user.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(usersResource, c.ns, userResource), &user.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*user.User), err
}

func (c *FakeUsers) Update(userResource *user.User) (result *user.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(usersResource, c.ns, userResource), &user.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*user.User), err
}

func (c *FakeUsers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(usersResource, c.ns, name), &user.User{})

	return err
}

func (c *FakeUsers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(usersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &user.UserList{})
	return err
}

func (c *FakeUsers) Get(name string, options v1.GetOptions) (result *user.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(usersResource, c.ns, name), &user.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*user.User), err
}

func (c *FakeUsers) List(opts v1.ListOptions) (result *user.UserList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(usersResource, usersKind, c.ns, opts), &user.UserList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &user.UserList{}
	for _, item := range obj.(*user.UserList).Items {
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

// Patch applies the patch and returns the patched userResource.
func (c *FakeUsers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *user.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(usersResource, c.ns, name, data, subresources...), &user.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*user.User), err
}
