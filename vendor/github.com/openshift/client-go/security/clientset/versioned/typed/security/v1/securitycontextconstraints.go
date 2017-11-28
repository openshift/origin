package v1

import (
	v1 "github.com/openshift/api/security/v1"
	scheme "github.com/openshift/client-go/security/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SecurityContextConstraintsesGetter has a method to return a SecurityContextConstraintsInterface.
// A group's client should implement this interface.
type SecurityContextConstraintsesGetter interface {
	SecurityContextConstraintses() SecurityContextConstraintsInterface
}

// SecurityContextConstraintsInterface has methods to work with SecurityContextConstraints resources.
type SecurityContextConstraintsInterface interface {
	Create(*v1.SecurityContextConstraints) (*v1.SecurityContextConstraints, error)
	Update(*v1.SecurityContextConstraints) (*v1.SecurityContextConstraints, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.SecurityContextConstraints, error)
	List(opts meta_v1.ListOptions) (*v1.SecurityContextConstraintsList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SecurityContextConstraints, err error)
	SecurityContextConstraintsExpansion
}

// securityContextConstraintses implements SecurityContextConstraintsInterface
type securityContextConstraintses struct {
	client rest.Interface
}

// newSecurityContextConstraintses returns a SecurityContextConstraintses
func newSecurityContextConstraintses(c *SecurityV1Client) *securityContextConstraintses {
	return &securityContextConstraintses{
		client: c.RESTClient(),
	}
}

// Get takes name of the securityContextConstraints, and returns the corresponding securityContextConstraints object, and an error if there is any.
func (c *securityContextConstraintses) Get(name string, options meta_v1.GetOptions) (result *v1.SecurityContextConstraints, err error) {
	result = &v1.SecurityContextConstraints{}
	err = c.client.Get().
		Resource("securitycontextconstraintses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SecurityContextConstraintses that match those selectors.
func (c *securityContextConstraintses) List(opts meta_v1.ListOptions) (result *v1.SecurityContextConstraintsList, err error) {
	result = &v1.SecurityContextConstraintsList{}
	err = c.client.Get().
		Resource("securitycontextconstraintses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested securityContextConstraintses.
func (c *securityContextConstraintses) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("securitycontextconstraintses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a securityContextConstraints and creates it.  Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *securityContextConstraintses) Create(securityContextConstraints *v1.SecurityContextConstraints) (result *v1.SecurityContextConstraints, err error) {
	result = &v1.SecurityContextConstraints{}
	err = c.client.Post().
		Resource("securitycontextconstraintses").
		Body(securityContextConstraints).
		Do().
		Into(result)
	return
}

// Update takes the representation of a securityContextConstraints and updates it. Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *securityContextConstraintses) Update(securityContextConstraints *v1.SecurityContextConstraints) (result *v1.SecurityContextConstraints, err error) {
	result = &v1.SecurityContextConstraints{}
	err = c.client.Put().
		Resource("securitycontextconstraintses").
		Name(securityContextConstraints.Name).
		Body(securityContextConstraints).
		Do().
		Into(result)
	return
}

// Delete takes name of the securityContextConstraints and deletes it. Returns an error if one occurs.
func (c *securityContextConstraintses) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("securitycontextconstraintses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *securityContextConstraintses) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("securitycontextconstraintses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *securityContextConstraintses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SecurityContextConstraints, err error) {
	result = &v1.SecurityContextConstraints{}
	err = c.client.Patch(pt).
		Resource("securitycontextconstraintses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
