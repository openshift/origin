package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjectRequests struct {
	Fake *Fake
}

var newProjectsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "newprojects"}

func (c *FakeProjectRequests) List(opts kapi.ListOptions) (*unversioned.Status, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(newProjectsResource, opts), &unversioned.Status{})
	if obj == nil {
		return nil, err
	}

	return obj.(*unversioned.Status), err
}

func (c *FakeProjectRequests) Create(inObj *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(newProjectsResource, inObj), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}
