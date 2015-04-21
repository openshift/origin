package client

import (
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type FakeProjectRequests struct {
	Fake *Fake
}

func (c *FakeProjectRequests) Create(project *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-newProject", Value: project}, &projectapi.ProjectRequest{})
	return obj.(*projectapi.Project), err
}
