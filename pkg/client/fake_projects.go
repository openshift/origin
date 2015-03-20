package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
)
import projectapi "github.com/openshift/origin/pkg/project/api"

type FakeProjects struct {
	Fake *Fake
}

func (c *FakeProjects) List(label labels.Selector, field fields.Selector) (*projectapi.ProjectList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-projects"})
	return &projectapi.ProjectList{}, nil
}

func (c *FakeProjects) Get(name string) (*projectapi.Project, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-project"})
	return &projectapi.Project{}, nil
}

func (c *FakeProjects) Create(project *projectapi.Project) (*projectapi.Project, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-project", Value: project})
	return &projectapi.Project{}, nil
}

func (c *FakeProjects) Update(project *projectapi.Project) (*projectapi.Project, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-project"})
	return &projectapi.Project{}, nil
}

func (c *FakeProjects) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-project", Value: name})
	return nil
}
