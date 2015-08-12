package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
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
