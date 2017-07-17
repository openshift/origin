package fake

import (
	v1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeUsers implements UserResourceInterface
type FakeUsers struct {
	Fake *FakeUserV1
	ns   string
}

var usersResource = schema.GroupVersionResource{Group: "user.openshift.io", Version: "v1", Resource: "users"}

var usersKind = schema.GroupVersionKind{Group: "user.openshift.io", Version: "v1", Kind: "User"}

func (c *FakeUsers) Create(userResource *v1.User) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(usersResource, c.ns, userResource), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) Update(userResource *v1.User) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(usersResource, c.ns, userResource), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(usersResource, c.ns, name), &v1.User{})

	return err
}

func (c *FakeUsers) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(usersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.UserList{})
	return err
}

func (c *FakeUsers) Get(name string, options meta_v1.GetOptions) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(usersResource, c.ns, name), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) List(opts meta_v1.ListOptions) (result *v1.UserList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(usersResource, usersKind, c.ns, opts), &v1.UserList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.UserList{}
	for _, item := range obj.(*v1.UserList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested users.
func (c *FakeUsers) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(usersResource, c.ns, opts))

}

// Patch applies the patch and returns the patched userResource.
func (c *FakeUsers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(usersResource, c.ns, name, data, subresources...), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}
