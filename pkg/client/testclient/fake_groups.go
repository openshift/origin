package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// FakeGroups implements GroupsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeGroups struct {
	Fake *Fake
}

var groupsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "groups"}
var groupsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Group"}

func (c *FakeGroups) Get(name string, options metav1.GetOptions) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(groupsResource, name), &userapi.Group{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) List(opts metav1.ListOptions) (*userapi.GroupList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(groupsResource, groupsKind, opts), &userapi.GroupList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.GroupList), err
}

func (c *FakeGroups) Create(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(groupsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Update(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(groupsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(groupsResource, name), &userapi.Group{})
	return err
}

func (c *FakeGroups) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(groupsResource, opts))
}
