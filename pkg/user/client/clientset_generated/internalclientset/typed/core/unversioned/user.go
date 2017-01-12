package unversioned

import (
	api "github.com/openshift/origin/pkg/user/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// UsersGetter has a method to return a UserInterface.
// A group's client should implement this interface.
type UsersGetter interface {
	Users(namespace string) UserInterface
}

// UserInterface has methods to work with User resources.
type UserInterface interface {
	Create(*api.User) (*api.User, error)
	Update(*api.User) (*api.User, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.User, error)
	List(opts pkg_api.ListOptions) (*api.UserList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.User, err error)
	UserExpansion
}

// users implements UserInterface
type users struct {
	client *CoreClient
	ns     string
}

// newUsers returns a Users
func newUsers(c *CoreClient, namespace string) *users {
	return &users{
		client: c,
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
func (c *users) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *users) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the user, and returns the corresponding user object, and an error if there is any.
func (c *users) Get(name string) (result *api.User, err error) {
	result = &api.User{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Users that match those selectors.
func (c *users) List(opts pkg_api.ListOptions) (result *api.UserList, err error) {
	result = &api.UserList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested users.
func (c *users) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched user.
func (c *users) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.User, err error) {
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
