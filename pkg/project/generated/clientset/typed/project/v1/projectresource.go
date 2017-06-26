package v1

import (
	v1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	scheme "github.com/openshift/origin/pkg/project/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ProjectsGetter has a method to return a ProjectResourceInterface.
// A group's client should implement this interface.
type ProjectsGetter interface {
	Projects() ProjectResourceInterface
}

// ProjectResourceInterface has methods to work with ProjectResource resources.
type ProjectResourceInterface interface {
	Create(*v1.Project) (*v1.Project, error)
	Update(*v1.Project) (*v1.Project, error)
	UpdateStatus(*v1.Project) (*v1.Project, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Project, error)
	List(opts meta_v1.ListOptions) (*v1.ProjectList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Project, err error)
	ProjectResourceExpansion
}

// projects implements ProjectResourceInterface
type projects struct {
	client rest.Interface
}

// newProjects returns a Projects
func newProjects(c *ProjectV1Client) *projects {
	return &projects{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a projectResource and creates it.  Returns the server's representation of the projectResource, and an error, if there is any.
func (c *projects) Create(projectResource *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Post().
		Resource("projects").
		Body(projectResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a projectResource and updates it. Returns the server's representation of the projectResource, and an error, if there is any.
func (c *projects) Update(projectResource *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Put().
		Resource("projects").
		Name(projectResource.Name).
		Body(projectResource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *projects) UpdateStatus(projectResource *v1.Project) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Put().
		Resource("projects").
		Name(projectResource.Name).
		SubResource("status").
		Body(projectResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the projectResource and deletes it. Returns an error if one occurs.
func (c *projects) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("projects").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *projects) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("projects").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the projectResource, and returns the corresponding projectResource object, and an error if there is any.
func (c *projects) Get(name string, options meta_v1.GetOptions) (result *v1.Project, err error) {
	result = &v1.Project{}
	err = c.client.Get().
		Resource("projects").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Projects that match those selectors.
func (c *projects) List(opts meta_v1.ListOptions) (result *v1.ProjectList, err error) {
	result = &v1.ProjectList{}
	err = c.client.Get().
		Resource("projects").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested projects.
func (c *projects) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("projects").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched projectResource.
func (c *projects) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Project, err error) {
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
