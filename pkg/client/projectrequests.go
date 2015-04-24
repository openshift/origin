package client

import (
	projectapi "github.com/openshift/origin/pkg/project/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UsersInterface has methods to work with User resources in a namespace
type ProjectRequestsInterface interface {
	ProjectRequests() ProjectRequestInterface
}

// UserInterface exposes methods on user resources.
type ProjectRequestInterface interface {
	Create(p *projectapi.ProjectRequest) (*projectapi.Project, error)
}

type newProjectRequestsStruct struct {
	r *Client
}

// newUsers returns a users
func newProjectRequests(c *Client) *newProjectRequestsStruct {
	return &newProjectRequestsStruct{
		r: c,
	}
}

// Create creates a new ProjectRequest
func (c *newProjectRequestsStruct) Create(p *projectapi.ProjectRequest) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Post().Resource("projectrequests").Body(p).Do().Into(result)
	return
}
