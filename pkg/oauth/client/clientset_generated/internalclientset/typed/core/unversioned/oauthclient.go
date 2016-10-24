package unversioned

import (
	api "github.com/openshift/origin/pkg/oauth/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// OAuthClientsGetter has a method to return a OAuthClientInterface.
// A group's client should implement this interface.
type OAuthClientsGetter interface {
	OAuthClients(namespace string) OAuthClientInterface
}

// OAuthClientInterface has methods to work with OAuthClient resources.
type OAuthClientInterface interface {
	Create(*api.OAuthClient) (*api.OAuthClient, error)
	Update(*api.OAuthClient) (*api.OAuthClient, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.OAuthClient, error)
	List(opts pkg_api.ListOptions) (*api.OAuthClientList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.OAuthClient, err error)
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
func (c *oAuthClients) Create(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("oauthclients").
		Body(oAuthClient).
		Do().
		Into(result)
	return
}

// Update takes the representation of a oAuthClient and updates it. Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *oAuthClients) Update(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
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
func (c *oAuthClients) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *oAuthClients) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the oAuthClient, and returns the corresponding oAuthClient object, and an error if there is any.
func (c *oAuthClients) Get(name string) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OAuthClients that match those selectors.
func (c *oAuthClients) List(opts pkg_api.ListOptions) (result *api.OAuthClientList, err error) {
	result = &api.OAuthClientList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested oAuthClients.
func (c *oAuthClients) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched oAuthClient.
func (c *oAuthClients) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
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
