package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUsers implements UsersInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

var usersResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "users"}

func (c *FakeUsers) Get(name string, options metav1.GetOptions) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(usersResource, name), &userapi.User{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) List(opts metainternal.ListOptions) (*userapi.UserList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(usersResource, opts), &userapi.UserList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserList), err
}

func (c *FakeUsers) Create(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(usersResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Update(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(usersResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(usersResource, name), nil)
	return err
}

func (c *FakeUsers) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(usersResource, opts))
}
