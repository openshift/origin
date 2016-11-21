package v1

import (
	v1 "github.com/openshift/origin/pkg/project/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ProjectsGetter has a method to return a ProjectInterface.
// A group's client should implement this interface.
type ProjectsGetter interface {
	Projects() ProjectInterface
}

// ProjectInterface has methods to work with Project resources.
type ProjectInterface interface {
	Create(*v1.Project) (*v1.Project, error)
	Update(*v1.Project) (*v1.Project, error)
	UpdateStatus(*v1.Project) (*v1.Project, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*v1.Project, error)
	List(opts api.ListOptions) (*v1.ProjectList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Project, err error)
	ProjectExpansion
}

// projects implements ProjectInterface
type projects struct {
	client *CoreClient
}

// newProjects returns a Projects
func newProjects(c *CoreClient) *projects {
	return &projects{
		client: c,
	}
}

// Create takes the representation of a project and creates it.  Returns the server's representation of the project, and an error, if there is any.
func (c *projects) Create(project *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Post().
		Resource("projects").
		Body(project).
		Do().
		Into(result)
	return
}

// Update takes the representation of a project and updates it. Returns the server's representation of the project, and an error, if there is any.
func (c *projects) Update(project *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Put().
		Resource("projects").
		Name(project.Name).
		Body(project).
		Do().
		Into(result)
	return
}

func (c *projects) UpdateStatus(project *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Put().
		Resource("projects").
		Name(project.Name).
		SubResource("status").
		Body(project).
		Do().
		Into(result)
	return
}

// Delete takes name of the project and deletes it. Returns an error if one occurs.
func (c *projects) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Resource("projects").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *projects) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Resource("projects").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the project, and returns the corresponding project object, and an error if there is any.
func (c *projects) Get(name string) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Get().
		Resource("projects").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Projects that match those selectors.
func (c *projects) List(opts api.ListOptions) (result *v1.ProjectList, err error) {
	result = &v1.ProjectList{}
	err = c.client.Get().
		Resource("projects").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested projects.
func (c *projects) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("projects").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched project.
func (c *projects) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Patch(pt).
		Resource("projects").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
