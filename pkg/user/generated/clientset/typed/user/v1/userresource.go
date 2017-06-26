package v1

import (
	v1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	scheme "github.com/openshift/origin/pkg/user/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Create(*v1.User) (*v1.User, error)
	Update(*v1.User) (*v1.User, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.User, error)
	List(opts meta_v1.ListOptions) (*v1.UserList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.User, err error)
	UserResourceExpansion
}

// users implements UserResourceInterface
type users struct {
	client rest.Interface
	ns     string
}

// newUsers returns a Users
func newUsers(c *UserV1Client, namespace string) *users {
	return &users{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a userResource and creates it.  Returns the server's representation of the userResource, and an error, if there is any.
func (c *users) Create(userResource *v1.User) (result *v1.User, err error) {
	result = &v1.User{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("users").
		Body(userResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a userResource and updates it. Returns the server's representation of the userResource, and an error, if there is any.
func (c *users) Update(userResource *v1.User) (result *v1.User, err error) {
	result = &v1.User{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("users").
		Name(userResource.Name).
		Body(userResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the userResource and deletes it. Returns an error if one occurs.
func (c *users) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *users) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the userResource, and returns the corresponding userResource object, and an error if there is any.
func (c *users) Get(name string, options meta_v1.GetOptions) (result *v1.User, err error) {
	result = &v1.User{}
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
func (c *users) List(opts meta_v1.ListOptions) (result *v1.UserList, err error) {
	result = &v1.UserList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested users.
func (c *users) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched userResource.
func (c *users) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.User, err error) {
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
