package v1

import (
	v1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	scheme "github.com/openshift/origin/pkg/project/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ProjectReservationsGetter has a method to return a ProjectReservationInterface.
// A group's client should implement this interface.
type ProjectReservationsGetter interface {
	ProjectReservations() ProjectReservationInterface
}

// ProjectReservationInterface has methods to work with ProjectReservation resources.
type ProjectReservationInterface interface {
	Create(*v1.ProjectReservation) (*v1.ProjectReservation, error)
	Update(*v1.ProjectReservation) (*v1.ProjectReservation, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.ProjectReservation, error)
	List(opts meta_v1.ListOptions) (*v1.ProjectReservationList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ProjectReservation, err error)
	ProjectReservationExpansion
}

// projectReservations implements ProjectReservationInterface
type projectReservations struct {
	client rest.Interface
}

// newProjectReservations returns a ProjectReservations
func newProjectReservations(c *ProjectV1Client) *projectReservations {
	return &projectReservations{
		client: c.RESTClient(),
	}
}

// Get takes name of the projectReservation, and returns the corresponding projectReservation object, and an error if there is any.
func (c *projectReservations) Get(name string, options meta_v1.GetOptions) (result *v1.ProjectReservation, err error) {
	result = &v1.ProjectReservation{}
	err = c.client.Get().
		Resource("projectreservations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ProjectReservations that match those selectors.
func (c *projectReservations) List(opts meta_v1.ListOptions) (result *v1.ProjectReservationList, err error) {
	result = &v1.ProjectReservationList{}
	err = c.client.Get().
		Resource("projectreservations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested projectReservations.
func (c *projectReservations) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("projectreservations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a projectReservation and creates it.  Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *projectReservations) Create(projectReservation *v1.ProjectReservation) (result *v1.ProjectReservation, err error) {
	result = &v1.ProjectReservation{}
	err = c.client.Post().
		Resource("projectreservations").
		Body(projectReservation).
		Do().
		Into(result)
	return
}

// Update takes the representation of a projectReservation and updates it. Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *projectReservations) Update(projectReservation *v1.ProjectReservation) (result *v1.ProjectReservation, err error) {
	result = &v1.ProjectReservation{}
	err = c.client.Put().
		Resource("projectreservations").
		Name(projectReservation.Name).
		Body(projectReservation).
		Do().
		Into(result)
	return
}

// Delete takes name of the projectReservation and deletes it. Returns an error if one occurs.
func (c *projectReservations) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("projectreservations").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *projectReservations) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("projectreservations").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched projectReservation.
func (c *projectReservations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ProjectReservation, err error) {
	result = &v1.ProjectReservation{}
	err = c.client.Patch(pt).
		Resource("projectreservations").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
