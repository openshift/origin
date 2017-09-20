package internalversion

import (
	user "github.com/openshift/origin/pkg/user/apis/user"
	scheme "github.com/openshift/origin/pkg/user/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// IdentitiesGetter has a method to return a IdentityInterface.
// A group's client should implement this interface.
type IdentitiesGetter interface {
	Identities() IdentityInterface
}

// IdentityInterface has methods to work with Identity resources.
type IdentityInterface interface {
	Create(*user.Identity) (*user.Identity, error)
	Update(*user.Identity) (*user.Identity, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*user.Identity, error)
	List(opts v1.ListOptions) (*user.IdentityList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *user.Identity, err error)
	IdentityExpansion
}

// identities implements IdentityInterface
type identities struct {
	client rest.Interface
}

// newIdentities returns a Identities
func newIdentities(c *UserClient) *identities {
	return &identities{
		client: c.RESTClient(),
	}
}

// Get takes name of the identity, and returns the corresponding identity object, and an error if there is any.
func (c *identities) Get(name string, options v1.GetOptions) (result *user.Identity, err error) {
	result = &user.Identity{}
	err = c.client.Get().
		Resource("identities").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Identities that match those selectors.
func (c *identities) List(opts v1.ListOptions) (result *user.IdentityList, err error) {
	result = &user.IdentityList{}
	err = c.client.Get().
		Resource("identities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested identities.
func (c *identities) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("identities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a identity and creates it.  Returns the server's representation of the identity, and an error, if there is any.
func (c *identities) Create(identity *user.Identity) (result *user.Identity, err error) {
	result = &user.Identity{}
	err = c.client.Post().
		Resource("identities").
		Body(identity).
		Do().
		Into(result)
	return
}

// Update takes the representation of a identity and updates it. Returns the server's representation of the identity, and an error, if there is any.
func (c *identities) Update(identity *user.Identity) (result *user.Identity, err error) {
	result = &user.Identity{}
	err = c.client.Put().
		Resource("identities").
		Name(identity.Name).
		Body(identity).
		Do().
		Into(result)
	return
}

// Delete takes name of the identity and deletes it. Returns an error if one occurs.
func (c *identities) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("identities").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *identities) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("identities").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched identity.
func (c *identities) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *user.Identity, err error) {
	result = &user.Identity{}
	err = c.client.Patch(pt).
		Resource("identities").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
