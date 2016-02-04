package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjectRequests struct {
	Fake *Fake
}

func (c *FakeProjectRequests) List(opts kapi.ListOptions) (*unversioned.Status, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("newprojects", opts), &unversioned.Status{})
	if obj == nil {
		return nil, err
	}

	return obj.(*unversioned.Status), err
}

func (c *FakeProjectRequests) Create(inObj *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("newprojects", inObj), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}
