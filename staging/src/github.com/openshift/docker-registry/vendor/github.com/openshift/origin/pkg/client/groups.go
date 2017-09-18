package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// GroupsInterface has methods to work with Group resources
type GroupsInterface interface {
	Groups() GroupInterface
}

// GroupInterface exposes methods on group resources.
type GroupInterface interface {
	List(opts metav1.ListOptions) (*userapi.GroupList, error)
	Get(name string, options metav1.GetOptions) (*userapi.Group, error)
	Create(group *userapi.Group) (*userapi.Group, error)
	Update(group *userapi.Group) (*userapi.Group, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

// groups implements GroupInterface interface
type groups struct {
	r *Client
}

// newGroups returns a groups
func newGroups(c *Client) *groups {
	return &groups{
		r: c,
	}
}

// List returns a list of groups that match the label and field selectors.
func (c *groups) List(opts metav1.ListOptions) (result *userapi.GroupList, err error) {
	result = &userapi.GroupList{}
	err = c.r.Get().
		Resource("groups").
		VersionedParams(&opts, kapi.ParameterCodec).
		Do().
		Into(result)
	return
}

// Get returns information about a particular group or an error
func (c *groups) Get(name string, options metav1.GetOptions) (result *userapi.Group, err error) {
	result = &userapi.Group{}
	err = c.r.Get().Resource("groups").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Create creates a new group. Returns the server's representation of the group and error if one occurs.
func (c *groups) Create(group *userapi.Group) (result *userapi.Group, err error) {
	result = &userapi.Group{}
	err = c.r.Post().Resource("groups").Body(group).Do().Into(result)
	return
}

// Update updates the group on server. Returns the server's representation of the group and error if one occurs.
func (c *groups) Update(group *userapi.Group) (result *userapi.Group, err error) {
	result = &userapi.Group{}
	err = c.r.Put().Resource("groups").Name(group.Name).Body(group).Do().Into(result)
	return
}

// Delete takes the name of the groups, and returns an error if one occurs during deletion of the groups
func (c *groups) Delete(name string) error {
	return c.r.Delete().Resource("groups").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested groups.
func (c *groups) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("groups").
		VersionedParams(&opts, kapi.ParameterCodec).
		Watch()
}
