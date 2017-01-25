package client

import (
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// OAuthAccessTokensInterface has methods to work with OAuthAccessTokens resources in a namespace
type OAuthAccessTokensInterface interface {
	OAuthAccessTokens() OAuthAccessTokenInterface
}

// OAuthAccessTokenInterface exposes methods on OAuthAccessTokens resources.
type OAuthAccessTokenInterface interface {
	Create(token *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error)
	Get(name string) (*oauthapi.OAuthAccessToken, error)
	List(opts kapi.ListOptions) (*oauthapi.OAuthAccessTokenList, error)
	Delete(name string) error
}

type oauthAccessTokenInterface struct {
	r *Client
}

func newOAuthAccessTokens(c *Client) *oauthAccessTokenInterface {
	return &oauthAccessTokenInterface{
		r: c,
	}
}

// Get returns information about a particular token and error if one occurs.
func (c *oauthAccessTokenInterface) Get(name string) (result *oauthapi.OAuthAccessToken, err error) {
	result = &oauthapi.OAuthAccessToken{}
	err = c.r.Get().Resource("oauthaccesstokens").Name(name).Do().Into(result)
	return
}

// List returns a list of tokens that match the label and field selectors.
func (c *oauthAccessTokenInterface) List(opts kapi.ListOptions) (result *oauthapi.OAuthAccessTokenList, err error) {
	result = &oauthapi.OAuthAccessTokenList{}
	err = c.r.Get().Resource("oauthaccesstokens").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

// Delete removes the OAuthAccessToken on server
func (c *oauthAccessTokenInterface) Delete(name string) (err error) {
	err = c.r.Delete().Resource("oauthaccesstokens").Name(name).Do().Error()
	return
}

func (c *oauthAccessTokenInterface) Create(token *oauthapi.OAuthAccessToken) (result *oauthapi.OAuthAccessToken, err error) {
	result = &oauthapi.OAuthAccessToken{}
	err = c.r.Post().Resource("oauthaccesstokens").Body(token).Do().Into(result)
	return
}
