package fake

import (
	v1 "github.com/openshift/api/project/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeProjectRequests implements ProjectRequestInterface
type FakeProjectRequests struct {
	Fake *FakeProjectV1
}

var projectrequestsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projectrequests"}

var projectrequestsKind = schema.GroupVersionKind{Group: "project.openshift.io", Version: "v1", Kind: "ProjectRequest"}

// Create takes the representation of a projectRequest and creates it.  Returns the server's representation of the project, and an error, if there is any.
func (c *FakeProjectRequests) Create(projectRequest *v1.ProjectRequest) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectrequestsResource, projectRequest), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}
