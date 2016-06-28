package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUsers implements UsersInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

func (c *FakeUsers) Get(name string) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("users", name), &userapi.User{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) List(opts kapi.ListOptions) (*userapi.UserList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("users", opts), &userapi.UserList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserList), err
}

func (c *FakeUsers) Create(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("users", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Update(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("users", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("users", name), nil)
	return err
}

func (c *FakeUsers) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("users", opts))
}
