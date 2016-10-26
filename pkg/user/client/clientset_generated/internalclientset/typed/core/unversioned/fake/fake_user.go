package fake

import (
	api "github.com/openshift/origin/pkg/user/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
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

var usersResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "users"}

func (c *FakeUsers) Create(user *api.User) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(usersResource, c.ns, user), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) Update(user *api.User) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(usersResource, c.ns, user), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(usersResource, c.ns, name), &api.User{})

	return err
}

func (c *FakeUsers) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(usersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.UserList{})
	return err
}

func (c *FakeUsers) Get(name string) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(usersResource, c.ns, name), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}

func (c *FakeUsers) List(opts pkg_api.ListOptions) (result *api.UserList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(usersResource, c.ns, opts), &api.UserList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeUsers) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(usersResource, c.ns, opts))

}

// Patch applies the patch and returns the patched user.
func (c *FakeUsers) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.User, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(usersResource, c.ns, name, data, subresources...), &api.User{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.User), err
}
