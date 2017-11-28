package v1

import (
	v1 "github.com/openshift/api/oauth/v1"
	scheme "github.com/openshift/client-go/oauth/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OAuthAccessTokensGetter has a method to return a OAuthAccessTokenInterface.
// A group's client should implement this interface.
type OAuthAccessTokensGetter interface {
	OAuthAccessTokens() OAuthAccessTokenInterface
}

// OAuthAccessTokenInterface has methods to work with OAuthAccessToken resources.
type OAuthAccessTokenInterface interface {
	Create(*v1.OAuthAccessToken) (*v1.OAuthAccessToken, error)
	Update(*v1.OAuthAccessToken) (*v1.OAuthAccessToken, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.OAuthAccessToken, error)
	List(opts meta_v1.ListOptions) (*v1.OAuthAccessTokenList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OAuthAccessToken, err error)
	OAuthAccessTokenExpansion
}

// oAuthAccessTokens implements OAuthAccessTokenInterface
type oAuthAccessTokens struct {
	client rest.Interface
}

// newOAuthAccessTokens returns a OAuthAccessTokens
func newOAuthAccessTokens(c *OauthV1Client) *oAuthAccessTokens {
	return &oAuthAccessTokens{
		client: c.RESTClient(),
	}
}

// Get takes name of the oAuthAccessToken, and returns the corresponding oAuthAccessToken object, and an error if there is any.
func (c *oAuthAccessTokens) Get(name string, options meta_v1.GetOptions) (result *v1.OAuthAccessToken, err error) {
	result = &v1.OAuthAccessToken{}
	err = c.client.Get().
		Resource("oauthaccesstokens").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OAuthAccessTokens that match those selectors.
func (c *oAuthAccessTokens) List(opts meta_v1.ListOptions) (result *v1.OAuthAccessTokenList, err error) {
	result = &v1.OAuthAccessTokenList{}
	err = c.client.Get().
		Resource("oauthaccesstokens").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested oAuthAccessTokens.
func (c *oAuthAccessTokens) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("oauthaccesstokens").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a oAuthAccessToken and creates it.  Returns the server's representation of the oAuthAccessToken, and an error, if there is any.
func (c *oAuthAccessTokens) Create(oAuthAccessToken *v1.OAuthAccessToken) (result *v1.OAuthAccessToken, err error) {
	result = &v1.OAuthAccessToken{}
	err = c.client.Post().
		Resource("oauthaccesstokens").
		Body(oAuthAccessToken).
		Do().
		Into(result)
	return
}

// Update takes the representation of a oAuthAccessToken and updates it. Returns the server's representation of the oAuthAccessToken, and an error, if there is any.
func (c *oAuthAccessTokens) Update(oAuthAccessToken *v1.OAuthAccessToken) (result *v1.OAuthAccessToken, err error) {
	result = &v1.OAuthAccessToken{}
	err = c.client.Put().
		Resource("oauthaccesstokens").
		Name(oAuthAccessToken.Name).
		Body(oAuthAccessToken).
		Do().
		Into(result)
	return
}

// Delete takes name of the oAuthAccessToken and deletes it. Returns an error if one occurs.
func (c *oAuthAccessTokens) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("oauthaccesstokens").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *oAuthAccessTokens) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("oauthaccesstokens").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched oAuthAccessToken.
func (c *oAuthAccessTokens) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OAuthAccessToken, err error) {
	result = &v1.OAuthAccessToken{}
	err = c.client.Patch(pt).
		Resource("oauthaccesstokens").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
