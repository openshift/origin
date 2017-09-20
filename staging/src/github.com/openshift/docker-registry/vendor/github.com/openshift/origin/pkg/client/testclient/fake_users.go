package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// FakeUsers implements UsersInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUsers struct {
	Fake *Fake
}

var usersResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "users"}
var usersKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "User"}

func (c *FakeUsers) Get(name string, options metav1.GetOptions) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(usersResource, name), &userapi.User{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) List(opts metav1.ListOptions) (*userapi.UserList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(usersResource, usersKind, opts), &userapi.UserList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserList), err
}

func (c *FakeUsers) Create(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(usersResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Update(inObj *userapi.User) (*userapi.User, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(usersResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.User), err
}

func (c *FakeUsers) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(usersResource, name), nil)
	return err
}

func (c *FakeUsers) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(usersResource, opts))
}
