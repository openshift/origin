package internalversion

import (
	api "github.com/openshift/origin/pkg/user/api"
	scheme "github.com/openshift/origin/pkg/user/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// UsersGetter has a method to return a UserResourceInterface.
// A group's client should implement this interface.
type UsersGetter interface {
	Users(namespace string) UserResourceInterface
}

// UserResourceInterface has methods to work with UserResource resources.
type UserResourceInterface interface {
	Create(*api.User) (*api.User, error)
	Update(*api.User) (*api.User, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*api.User, error)
	List(opts v1.ListOptions) (*api.UserList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.User, err error)
	UserResourceExpansion
}

// users implements UserResourceInterface
type users struct {
	client rest.Interface
	ns     string
}

// newUsers returns a Users
func newUsers(c *UserClient, namespace string) *users {
	return &users{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a user and creates it.  Returns the server's representation of the user, and an error, if there is any.
func (c *users) Create(user *api.User) (result *api.User, err error) {
	result = &api.User{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("users").
		Body(user).
		Do().
		Into(result)
	return
}

// Update takes the representation of a user and updates it. Returns the server's representation of the user, and an error, if there is any.
func (c *users) Update(user *api.User) (result *api.User, err error) {
	result = &api.User{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("users").
		Name(user.Name).
		Body(user).
		Do().
		Into(result)
	return
}

// Delete takes name of the user and deletes it. Returns an error if one occurs.
func (c *users) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *users) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the user, and returns the corresponding user object, and an error if there is any.
func (c *users) Get(name string, options v1.GetOptions) (result *api.User, err error) {
	result = &api.User{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Users that match those selectors.
func (c *users) List(opts v1.ListOptions) (result *api.UserList, err error) {
	result = &api.UserList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested users.
func (c *users) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched user.
func (c *users) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.User, err error) {
	result = &api.User{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("users").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
