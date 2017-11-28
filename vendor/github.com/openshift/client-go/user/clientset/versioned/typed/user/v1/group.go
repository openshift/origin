package v1

import (
	v1 "github.com/openshift/api/user/v1"
	scheme "github.com/openshift/client-go/user/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// GroupsGetter has a method to return a GroupInterface.
// A group's client should implement this interface.
type GroupsGetter interface {
	Groups() GroupInterface
}

// GroupInterface has methods to work with Group resources.
type GroupInterface interface {
	Create(*v1.Group) (*v1.Group, error)
	Update(*v1.Group) (*v1.Group, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Group, error)
	List(opts meta_v1.ListOptions) (*v1.GroupList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Group, err error)
	GroupExpansion
}

// groups implements GroupInterface
type groups struct {
	client rest.Interface
}

// newGroups returns a Groups
func newGroups(c *UserV1Client) *groups {
	return &groups{
		client: c.RESTClient(),
	}
}

// Get takes name of the group, and returns the corresponding group object, and an error if there is any.
func (c *groups) Get(name string, options meta_v1.GetOptions) (result *v1.Group, err error) {
	result = &v1.Group{}
	err = c.client.Get().
		Resource("groups").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Groups that match those selectors.
func (c *groups) List(opts meta_v1.ListOptions) (result *v1.GroupList, err error) {
	result = &v1.GroupList{}
	err = c.client.Get().
		Resource("groups").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested groups.
func (c *groups) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("groups").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a group and creates it.  Returns the server's representation of the group, and an error, if there is any.
func (c *groups) Create(group *v1.Group) (result *v1.Group, err error) {
	result = &v1.Group{}
	err = c.client.Post().
		Resource("groups").
		Body(group).
		Do().
		Into(result)
	return
}

// Update takes the representation of a group and updates it. Returns the server's representation of the group, and an error, if there is any.
func (c *groups) Update(group *v1.Group) (result *v1.Group, err error) {
	result = &v1.Group{}
	err = c.client.Put().
		Resource("groups").
		Name(group.Name).
		Body(group).
		Do().
		Into(result)
	return
}

// Delete takes name of the group and deletes it. Returns an error if one occurs.
func (c *groups) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("groups").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *groups) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("groups").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched group.
func (c *groups) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Group, err error) {
	result = &v1.Group{}
	err = c.client.Patch(pt).
		Resource("groups").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
