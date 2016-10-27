package v1

import (
	v1 "github.com/openshift/origin/pkg/oauth/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// OAuthClientsGetter has a method to return a OAuthClientInterface.
// A group's client should implement this interface.
type OAuthClientsGetter interface {
	OAuthClients(namespace string) OAuthClientInterface
}

// OAuthClientInterface has methods to work with OAuthClient resources.
type OAuthClientInterface interface {
	Create(*v1.OAuthClient) (*v1.OAuthClient, error)
	Update(*v1.OAuthClient) (*v1.OAuthClient, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*v1.OAuthClient, error)
	List(opts api.ListOptions) (*v1.OAuthClientList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.OAuthClient, err error)
	OAuthClientExpansion
}

// oAuthClients implements OAuthClientInterface
type oAuthClients struct {
	client *CoreClient
	ns     string
}

// newOAuthClients returns a OAuthClients
func newOAuthClients(c *CoreClient, namespace string) *oAuthClients {
	return &oAuthClients{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a oAuthClient and creates it.  Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *oAuthClients) Create(oAuthClient *v1.OAuthClient) (result *v1.OAuthClient, err error) {
	result = &v1.OAuthClient{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("oauthclients").
		Body(oAuthClient).
		Do().
		Into(result)
	return
}

// Update takes the representation of a oAuthClient and updates it. Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *oAuthClients) Update(oAuthClient *v1.OAuthClient) (result *v1.OAuthClient, err error) {
	result = &v1.OAuthClient{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(oAuthClient.Name).
		Body(oAuthClient).
		Do().
		Into(result)
	return
}

// Delete takes name of the oAuthClient and deletes it. Returns an error if one occurs.
func (c *oAuthClients) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *oAuthClients) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the oAuthClient, and returns the corresponding oAuthClient object, and an error if there is any.
func (c *oAuthClients) Get(name string) (result *v1.OAuthClient, err error) {
	result = &v1.OAuthClient{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OAuthClients that match those selectors.
func (c *oAuthClients) List(opts api.ListOptions) (result *v1.OAuthClientList, err error) {
	result = &v1.OAuthClientList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested oAuthClients.
func (c *oAuthClients) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched oAuthClient.
func (c *oAuthClients) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.OAuthClient, err error) {
	result = &v1.OAuthClient{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("oauthclients").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
