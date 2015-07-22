package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjects implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjects struct {
	Fake *Fake
}

func (c *FakeProjects) List(label labels.Selector, field fields.Selector) (*projectapi.ProjectList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-projects"}, &projectapi.ProjectList{})
	return obj.(*projectapi.ProjectList), err
}

func (c *FakeProjects) Get(name string) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-project"}, &projectapi.Project{})
	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Create(project *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-project", Value: project}, &projectapi.Project{})
	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Update(project *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-project"}, &projectapi.Project{})
	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-project", Value: name})
	return nil
}
