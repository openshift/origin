package fake

import (
	v1 "github.com/openshift/origin/pkg/user/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeUsers implements UserInterface
type FakeUsers struct {
	Fake *FakeCore
	ns   string
}

var usersResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "users"}

func (c *FakeUsers) Create(user *v1.User) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(usersResource, c.ns, user), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) Update(user *v1.User) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(usersResource, c.ns, user), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(usersResource, c.ns, name), &v1.User{})

	return err
}

func (c *FakeUsers) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(usersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.UserList{})
	return err
}

func (c *FakeUsers) Get(name string) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(usersResource, c.ns, name), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}

func (c *FakeUsers) List(opts api.ListOptions) (result *v1.UserList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(usersResource, c.ns, opts), &v1.UserList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeUsers) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(usersResource, c.ns, opts))

}

// Patch applies the patch and returns the patched user.
func (c *FakeUsers) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(usersResource, c.ns, name, data, subresources...), &v1.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.User), err
}
