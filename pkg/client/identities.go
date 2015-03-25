package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UsersInterface has methods to work with User resources in a namespace
type IdentitiesInterface interface {
	Identities() IdentityInterface
}

// UserInterface exposes methods on user resources.
type IdentityInterface interface {
	List(label labels.Selector, field fields.Selector) (*userapi.IdentityList, error)
	Get(name string) (*userapi.Identity, error)
	Create(identity *userapi.Identity) (*userapi.Identity, error)
	Update(identity *userapi.Identity) (*userapi.Identity, error)
}

// users implements UserIdentityMappingsNamespacer interface
type identities struct {
	r *Client
}

// newUsers returns a users
func newIdentities(c *Client) *identities {
	return &identities{
		r: c,
	}
}

// List returns a list of users that match the label and field selectors.
func (c *identities) List(label labels.Selector, field fields.Selector) (result *userapi.IdentityList, err error) {
	result = &userapi.IdentityList{}
	err = c.r.Get().
		Resource("identities").
		LabelsSelectorParam("labels", label).
		FieldsSelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular user or an error
func (c *identities) Get(name string) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Get().Resource("identities").Name(name).Do().Into(result)
	return
}

// Create creates a new user. Returns the server's representation of the user and error if one occurs.
func (c *identities) Create(user *userapi.Identity) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Post().Resource("identities").Body(user).Do().Into(result)
	return
}

// Update updates the user on server. Returns the server's representation of the user and error if one occurs.
func (c *identities) Update(user *userapi.Identity) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Put().Resource("identities").Name(user.Name).Body(user).Do().Into(result)
	return
}
