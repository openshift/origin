package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

// ProjectsInterface has methods to work with Project resources in a namespace
type ProjectsInterface interface {
	Projects() ProjectInterface
}

// ProjectInterface exposes methods on project resources.
type ProjectInterface interface {
	Create(p *projectapi.Project) (*projectapi.Project, error)
	Update(p *projectapi.Project) (*projectapi.Project, error)
	Delete(name string) error
	Get(name string, options metav1.GetOptions) (*projectapi.Project, error)
	List(opts metav1.ListOptions) (*projectapi.ProjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
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
func (c *projects) Get(name string, options metav1.GetOptions) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Get().Resource("projects").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// List returns all projects matching the label selector
func (c *projects) List(opts metav1.ListOptions) (result *projectapi.ProjectList, err error) {
	result = &projectapi.ProjectList{}
	err = c.r.Get().
		Resource("projects").
		VersionedParams(&opts, kapi.ParameterCodec).
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

// Watch returns a watch.Interface that watches the requested namespaces.
func (c *projects) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("projects").
		VersionedParams(&opts, kapi.ParameterCodec).
		Watch()
}
