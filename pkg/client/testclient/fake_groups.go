package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeGroups implements GroupsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeGroups struct {
	Fake *Fake
}

var groupsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "groups"}

func (c *FakeGroups) Get(name string, options metav1.GetOptions) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(groupsResource, name), &userapi.Group{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) List(opts metainternal.ListOptions) (*userapi.GroupList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewRootListAction(groupsResource, optsv1), &userapi.GroupList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.GroupList), err
}

func (c *FakeGroups) Create(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(groupsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Update(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(groupsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(groupsResource, name), &userapi.Group{})
	return err
}

func (c *FakeGroups) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(core.NewRootWatchAction(groupsResource, optsv1))
}
