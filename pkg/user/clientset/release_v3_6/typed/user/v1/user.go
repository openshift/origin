package v1

import (
	v1 "github.com/openshift/origin/pkg/user/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// UsersGetter has a method to return a UserResourceInterface.
// A group's client should implement this interface.
type UsersGetter interface {
	Users(namespace string) UserResourceInterface
}

// UserResourceInterface has methods to work with UserResource resources.
type UserResourceInterface interface {
	Create(*v1.User) (*v1.User, error)
	Update(*v1.User) (*v1.User, error)
	Delete(name string, options *api_v1.DeleteOptions) error
	DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error
	Get(name string) (*v1.User, error)
	List(opts api_v1.ListOptions) (*v1.UserList, error)
	Watch(opts api_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.User, err error)
	UserResourceExpansion
}

// users implements UserResourceInterface
type users struct {
	client restclient.Interface
	ns     string
}

// newUsers returns a Users
func newUsers(c *UserV1Client, namespace string) *users {
	return &users{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a user and creates it.  Returns the server's representation of the user, and an error, if there is any.
func (c *users) Create(user *v1.User) (result *v1.User, err error) {
	result = &v1.User{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("users").
		Body(user).
		Do().
		Into(result)
	return
}

// Update takes the representation of a user and updates it. Returns the server's representation of the user, and an error, if there is any.
func (c *users) Update(user *v1.User) (result *v1.User, err error) {
	result = &v1.User{}
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
func (c *users) Delete(name string, options *api_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *users) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the user, and returns the corresponding user object, and an error if there is any.
func (c *users) Get(name string) (result *v1.User, err error) {
	result = &v1.User{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Users that match those selectors.
func (c *users) List(opts api_v1.ListOptions) (result *v1.UserList, err error) {
	result = &v1.UserList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested users.
func (c *users) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched user.
func (c *users) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.User, err error) {
	result = &v1.User{}
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
