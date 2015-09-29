package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeGroups implements GroupsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeGroups struct {
	Fake *Fake
}

func (c *FakeGroups) Get(name string) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("groups", name), &userapi.Group{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) List(label labels.Selector, field fields.Selector) (*userapi.GroupList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("groups", label, field), &userapi.GroupList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.GroupList), err
}

func (c *FakeGroups) Create(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("groups", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Update(inObj *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("groups", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("groups", name), &userapi.Group{})
	return err
}
