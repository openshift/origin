package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	scheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// RoleBindingRestrictionsGetter has a method to return a RoleBindingRestrictionInterface.
// A group's client should implement this interface.
type RoleBindingRestrictionsGetter interface {
	RoleBindingRestrictions(namespace string) RoleBindingRestrictionInterface
}

// RoleBindingRestrictionInterface has methods to work with RoleBindingRestriction resources.
type RoleBindingRestrictionInterface interface {
	Create(*authorization.RoleBindingRestriction) (*authorization.RoleBindingRestriction, error)
	Update(*authorization.RoleBindingRestriction) (*authorization.RoleBindingRestriction, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*authorization.RoleBindingRestriction, error)
	List(opts v1.ListOptions) (*authorization.RoleBindingRestrictionList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.RoleBindingRestriction, err error)
	RoleBindingRestrictionExpansion
}

// roleBindingRestrictions implements RoleBindingRestrictionInterface
type roleBindingRestrictions struct {
	client rest.Interface
	ns     string
}

// newRoleBindingRestrictions returns a RoleBindingRestrictions
func newRoleBindingRestrictions(c *AuthorizationClient, namespace string) *roleBindingRestrictions {
	return &roleBindingRestrictions{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the roleBindingRestriction, and returns the corresponding roleBindingRestriction object, and an error if there is any.
func (c *roleBindingRestrictions) Get(name string, options v1.GetOptions) (result *authorization.RoleBindingRestriction, err error) {
	result = &authorization.RoleBindingRestriction{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of RoleBindingRestrictions that match those selectors.
func (c *roleBindingRestrictions) List(opts v1.ListOptions) (result *authorization.RoleBindingRestrictionList, err error) {
	result = &authorization.RoleBindingRestrictionList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested roleBindingRestrictions.
func (c *roleBindingRestrictions) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a roleBindingRestriction and creates it.  Returns the server's representation of the roleBindingRestriction, and an error, if there is any.
func (c *roleBindingRestrictions) Create(roleBindingRestriction *authorization.RoleBindingRestriction) (result *authorization.RoleBindingRestriction, err error) {
	result = &authorization.RoleBindingRestriction{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		Body(roleBindingRestriction).
		Do().
		Into(result)
	return
}

// Update takes the representation of a roleBindingRestriction and updates it. Returns the server's representation of the roleBindingRestriction, and an error, if there is any.
func (c *roleBindingRestrictions) Update(roleBindingRestriction *authorization.RoleBindingRestriction) (result *authorization.RoleBindingRestriction, err error) {
	result = &authorization.RoleBindingRestriction{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		Name(roleBindingRestriction.Name).
		Body(roleBindingRestriction).
		Do().
		Into(result)
	return
}

// Delete takes name of the roleBindingRestriction and deletes it. Returns an error if one occurs.
func (c *roleBindingRestrictions) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *roleBindingRestrictions) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched roleBindingRestriction.
func (c *roleBindingRestrictions) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.RoleBindingRestriction, err error) {
	result = &authorization.RoleBindingRestriction{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("rolebindingrestrictions").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
