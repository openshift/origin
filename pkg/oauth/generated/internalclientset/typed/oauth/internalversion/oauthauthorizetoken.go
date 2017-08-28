package internalversion

import (
	oauth "github.com/openshift/origin/pkg/oauth/apis/oauth"
	scheme "github.com/openshift/origin/pkg/oauth/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OAuthAuthorizeTokensGetter has a method to return a OAuthAuthorizeTokenInterface.
// A group's client should implement this interface.
type OAuthAuthorizeTokensGetter interface {
	OAuthAuthorizeTokens() OAuthAuthorizeTokenInterface
}

// OAuthAuthorizeTokenInterface has methods to work with OAuthAuthorizeToken resources.
type OAuthAuthorizeTokenInterface interface {
	Create(*oauth.OAuthAuthorizeToken) (*oauth.OAuthAuthorizeToken, error)
	Update(*oauth.OAuthAuthorizeToken) (*oauth.OAuthAuthorizeToken, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*oauth.OAuthAuthorizeToken, error)
	List(opts v1.ListOptions) (*oauth.OAuthAuthorizeTokenList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *oauth.OAuthAuthorizeToken, err error)
	OAuthAuthorizeTokenExpansion
}

// oAuthAuthorizeTokens implements OAuthAuthorizeTokenInterface
type oAuthAuthorizeTokens struct {
	client rest.Interface
}

// newOAuthAuthorizeTokens returns a OAuthAuthorizeTokens
func newOAuthAuthorizeTokens(c *OauthClient) *oAuthAuthorizeTokens {
	return &oAuthAuthorizeTokens{
		client: c.RESTClient(),
	}
}

// Get takes name of the oAuthAuthorizeToken, and returns the corresponding oAuthAuthorizeToken object, and an error if there is any.
func (c *oAuthAuthorizeTokens) Get(name string, options v1.GetOptions) (result *oauth.OAuthAuthorizeToken, err error) {
	result = &oauth.OAuthAuthorizeToken{}
	err = c.client.Get().
		Resource("oauthauthorizetokens").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OAuthAuthorizeTokens that match those selectors.
func (c *oAuthAuthorizeTokens) List(opts v1.ListOptions) (result *oauth.OAuthAuthorizeTokenList, err error) {
	result = &oauth.OAuthAuthorizeTokenList{}
	err = c.client.Get().
		Resource("oauthauthorizetokens").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested oAuthAuthorizeTokens.
func (c *oAuthAuthorizeTokens) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("oauthauthorizetokens").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a oAuthAuthorizeToken and creates it.  Returns the server's representation of the oAuthAuthorizeToken, and an error, if there is any.
func (c *oAuthAuthorizeTokens) Create(oAuthAuthorizeToken *oauth.OAuthAuthorizeToken) (result *oauth.OAuthAuthorizeToken, err error) {
	result = &oauth.OAuthAuthorizeToken{}
	err = c.client.Post().
		Resource("oauthauthorizetokens").
		Body(oAuthAuthorizeToken).
		Do().
		Into(result)
	return
}

// Update takes the representation of a oAuthAuthorizeToken and updates it. Returns the server's representation of the oAuthAuthorizeToken, and an error, if there is any.
func (c *oAuthAuthorizeTokens) Update(oAuthAuthorizeToken *oauth.OAuthAuthorizeToken) (result *oauth.OAuthAuthorizeToken, err error) {
	result = &oauth.OAuthAuthorizeToken{}
	err = c.client.Put().
		Resource("oauthauthorizetokens").
		Name(oAuthAuthorizeToken.Name).
		Body(oAuthAuthorizeToken).
		Do().
		Into(result)
	return
}

// Delete takes name of the oAuthAuthorizeToken and deletes it. Returns an error if one occurs.
func (c *oAuthAuthorizeTokens) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("oauthauthorizetokens").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *oAuthAuthorizeTokens) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("oauthauthorizetokens").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched oAuthAuthorizeToken.
func (c *oAuthAuthorizeTokens) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *oauth.OAuthAuthorizeToken, err error) {
	result = &oauth.OAuthAuthorizeToken{}
	err = c.client.Patch(pt).
		Resource("oauthauthorizetokens").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
