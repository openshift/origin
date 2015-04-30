package client

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

type FakeProjectRequests struct {
	Fake *Fake
}

func (c *FakeProjectRequests) Create(project *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-newProject", Value: project}, &projectapi.ProjectRequest{})
	return obj.(*projectapi.Project), err
}

func (c *FakeProjectRequests) List(label labels.Selector, field fields.Selector) (*kapi.Status, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-newProject"}, &kapi.Status{})
	return obj.(*kapi.Status), err
}
