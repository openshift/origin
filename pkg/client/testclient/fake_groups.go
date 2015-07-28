package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeGroups implements GroupsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeGroups struct {
	Fake *Fake
}

func (c *FakeGroups) List(label labels.Selector, field fields.Selector) (*userapi.GroupList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-groups"}, &userapi.GroupList{})
	return obj.(*userapi.GroupList), err
}

func (c *FakeGroups) Get(name string) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-group", Value: name}, &userapi.Group{})
	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Create(group *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-group", Value: group}, &userapi.Group{})
	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Update(group *userapi.Group) (*userapi.Group, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-group", Value: group}, &userapi.Group{})
	return obj.(*userapi.Group), err
}

func (c *FakeGroups) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-group"})
	return nil
}
