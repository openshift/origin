package internalversion

import (
	project "github.com/openshift/origin/pkg/project/apis/project"
	rest "k8s.io/client-go/rest"
)

// ProjectRequestsGetter has a method to return a ProjectRequestInterface.
// A group's client should implement this interface.
type ProjectRequestsGetter interface {
	ProjectRequests() ProjectRequestInterface
}

// ProjectRequestInterface has methods to work with ProjectRequest resources.
type ProjectRequestInterface interface {
	Create(*project.ProjectRequest) (*project.Project, error)

	ProjectRequestExpansion
}

// projectRequests implements ProjectRequestInterface
type projectRequests struct {
	client rest.Interface
}

// newProjectRequests returns a ProjectRequests
func newProjectRequests(c *ProjectClient) *projectRequests {
	return &projectRequests{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a projectRequest and creates it.  Returns the server's representation of the projectResource, and an error, if there is any.
func (c *projectRequests) Create(projectRequest *project.ProjectRequest) (result *project.Project, err error) {
	result = &project.Project{}
	err = c.client.Post().
		Resource("projectrequests").
		Body(projectRequest).
		Do().
		Into(result)
	return
}
