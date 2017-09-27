package fake

import (
	project "github.com/openshift/origin/pkg/project/apis/project"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeProjectRequests implements ProjectRequestInterface
type FakeProjectRequests struct {
	Fake *FakeProject
}

var projectrequestsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "", Resource: "projectrequests"}

var projectrequestsKind = schema.GroupVersionKind{Group: "project.openshift.io", Version: "", Kind: "ProjectRequest"}

// Create takes the representation of a projectRequest and creates it.  Returns the server's representation of the projectResource, and an error, if there is any.
func (c *FakeProjectRequests) Create(projectRequest *project.ProjectRequest) (result *project.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectrequestsResource, projectRequest), &project.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project.Project), err
}
