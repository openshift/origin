package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjectRequests struct {
	Fake *Fake
}

func (c *FakeProjectRequests) List(label labels.Selector, field fields.Selector) (*unversioned.Status, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("newprojects", label, field), &unversioned.Status{})
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
