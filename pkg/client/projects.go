package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

// ProjectsInterface has methods to work with Project resources in a namespace
type ProjectsInterface interface {
	Projects() ProjectInterface
}

// ProjectInterface exposes methods on project resources.
type ProjectInterface interface {
	Create(p *projectapi.Project) (*projectapi.Project, error)
	Delete(name string) error
	Get(name string) (*projectapi.Project, error)
	List(label labels.Selector, field fields.Selector) (*projectapi.ProjectList, error)
}

type projects struct {
	r *Client
}

// newUsers returns a project
func newProjects(c *Client) *projects {
	return &projects{
		r: c,
	}
}

// Get returns information about a particular project or an error
func (c *projects) Get(name string) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Get().Resource("projects").Name(name).Do().Into(result)
	return
}

// List returns all projects matching the label selector
func (c *projects) List(label labels.Selector, field fields.Selector) (result *projectapi.ProjectList, err error) {
	result = &projectapi.ProjectList{}
	err = c.r.Get().
		Resource("projects").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Create creates a new Project
func (c *projects) Create(p *projectapi.Project) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Post().Resource("projects").Body(p).Do().Into(result)
	return
}

// Update updates the project on server
func (c *projects) Update(p *projectapi.Project) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Put().Resource("projects").Name(p.Name).Body(p).Do().Into(result)
	return
}

// Delete removes the project on server
func (c *projects) Delete(name string) (err error) {
	err = c.r.Delete().Resource("projects").Name(name).Do().Error()
	return
}
