package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// IdentitiesInterface has methods to work with Identity resources
type IdentitiesInterface interface {
	Identities() IdentityInterface
}

// IdentityInterface exposes methods on identity resources.
type IdentityInterface interface {
	List(label labels.Selector, field fields.Selector) (*userapi.IdentityList, error)
	Get(name string) (*userapi.Identity, error)
	Create(identity *userapi.Identity) (*userapi.Identity, error)
	Update(identity *userapi.Identity) (*userapi.Identity, error)
}

// identities implements IdentityInterface interface
type identities struct {
	r *Client
}

// newIdentities returns an identities client
func newIdentities(c *Client) *identities {
	return &identities{
		r: c,
	}
}

// List returns a list of identities that match the label and field selectors.
func (c *identities) List(label labels.Selector, field fields.Selector) (result *userapi.IdentityList, err error) {
	result = &userapi.IdentityList{}
	err = c.r.Get().
		Resource("identities").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular identity or an error
func (c *identities) Get(name string) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Get().Resource("identities").Name(name).Do().Into(result)
	return
}

// Create creates a new identity. Returns the server's representation of the identity and error if one occurs.
func (c *identities) Create(identity *userapi.Identity) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Post().Resource("identities").Body(identity).Do().Into(result)
	return
}

// Update updates the identity on server. Returns the server's representation of the identity and error if one occurs.
func (c *identities) Update(identity *userapi.Identity) (result *userapi.Identity, err error) {
	result = &userapi.Identity{}
	err = c.r.Put().Resource("identities").Name(identity.Name).Body(identity).Do().Into(result)
	return
}
