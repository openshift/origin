package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// UsersInterface has methods to work with User resources
type UsersInterface interface {
	Users() UserInterface
}

// UserInterface exposes methods on user resources.
type UserInterface interface {
	List(label labels.Selector, field fields.Selector) (*userapi.UserList, error)
	Get(name string) (*userapi.User, error)
	Create(user *userapi.User) (*userapi.User, error)
	Update(user *userapi.User) (*userapi.User, error)
}

// UsersImpersonator has methods to work with User resources, acting as the
// user specified by token.
type UsersImpersonator interface {
	ImpersonateUsers(token string) UserInterface
}

// users implements UserInterface interface
type users struct {
	r     resource.RESTClient
	token string
}

// newUsers returns a users
func newUsers(c *Client, token string) *users {
	return &users{
		r:     c,
		token: token,
	}
}

// List returns a list of users that match the label and field selectors.
func (c *users) List(label labels.Selector, field fields.Selector) (result *userapi.UserList, err error) {
	result = &userapi.UserList{}
	err = overrideAuth(c.token, c.r.Get().
		Resource("users").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field)).
		Do().
		Into(result)
	return
}

// Get returns information about a particular user or an error
func (c *users) Get(name string) (result *userapi.User, err error) {
	result = &userapi.User{}
	err = overrideAuth(c.token, c.r.Get().Resource("users").Name(name)).Do().Into(result)
	return
}

// Create creates a new user. Returns the server's representation of the user and error if one occurs.
func (c *users) Create(user *userapi.User) (result *userapi.User, err error) {
	result = &userapi.User{}
	err = overrideAuth(c.token, c.r.Post().Resource("users")).Body(user).Do().Into(result)
	return
}

// Update updates the user on server. Returns the server's representation of the user and error if one occurs.
func (c *users) Update(user *userapi.User) (result *userapi.User, err error) {
	result = &userapi.User{}
	err = overrideAuth(c.token, c.r.Put().Resource("users").Name(user.Name)).Body(user).Do().Into(result)
	return
}
