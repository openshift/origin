package v1

import (
	v1 "github.com/openshift/api/user/v1"
	scheme "github.com/openshift/client-go/user/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// UserIdentityMappingsGetter has a method to return a UserIdentityMappingInterface.
// A group's client should implement this interface.
type UserIdentityMappingsGetter interface {
	UserIdentityMappings() UserIdentityMappingInterface
}

// UserIdentityMappingInterface has methods to work with UserIdentityMapping resources.
type UserIdentityMappingInterface interface {
	Create(*v1.UserIdentityMapping) (*v1.UserIdentityMapping, error)
	Update(*v1.UserIdentityMapping) (*v1.UserIdentityMapping, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.UserIdentityMapping, error)
	UserIdentityMappingExpansion
}

// userIdentityMappings implements UserIdentityMappingInterface
type userIdentityMappings struct {
	client rest.Interface
}

// newUserIdentityMappings returns a UserIdentityMappings
func newUserIdentityMappings(c *UserV1Client) *userIdentityMappings {
	return &userIdentityMappings{
		client: c.RESTClient(),
	}
}

// Get takes name of the userIdentityMapping, and returns the corresponding userIdentityMapping object, and an error if there is any.
func (c *userIdentityMappings) Get(name string, options meta_v1.GetOptions) (result *v1.UserIdentityMapping, err error) {
	result = &v1.UserIdentityMapping{}
	err = c.client.Get().
		Resource("useridentitymappings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Create takes the representation of a userIdentityMapping and creates it.  Returns the server's representation of the userIdentityMapping, and an error, if there is any.
func (c *userIdentityMappings) Create(userIdentityMapping *v1.UserIdentityMapping) (result *v1.UserIdentityMapping, err error) {
	result = &v1.UserIdentityMapping{}
	err = c.client.Post().
		Resource("useridentitymappings").
		Body(userIdentityMapping).
		Do().
		Into(result)
	return
}

// Update takes the representation of a userIdentityMapping and updates it. Returns the server's representation of the userIdentityMapping, and an error, if there is any.
func (c *userIdentityMappings) Update(userIdentityMapping *v1.UserIdentityMapping) (result *v1.UserIdentityMapping, err error) {
	result = &v1.UserIdentityMapping{}
	err = c.client.Put().
		Resource("useridentitymappings").
		Name(userIdentityMapping.Name).
		Body(userIdentityMapping).
		Do().
		Into(result)
	return
}

// Delete takes name of the userIdentityMapping and deletes it. Returns an error if one occurs.
func (c *userIdentityMappings) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("useridentitymappings").
		Name(name).
		Body(options).
		Do().
		Error()
}
