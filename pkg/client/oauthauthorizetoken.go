package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type OAuthAuthorizeTokensInterface interface {
	OAuthAuthorizeTokens() OAuthAuthorizeTokenInterface
}

type OAuthAuthorizeTokenInterface interface {
	Create(token *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error)
	Get(name string, options metav1.GetOptions) (*oauthapi.OAuthAuthorizeToken, error)
	Delete(name string) error
}

type oauthAuthorizeTokenInterface struct {
	r *Client
}

func newOAuthAuthorizeTokens(c *Client) *oauthAuthorizeTokenInterface {
	return &oauthAuthorizeTokenInterface{
		r: c,
	}
}

func (c *oauthAuthorizeTokenInterface) Delete(name string) (err error) {
	err = c.r.Delete().Resource("oauthauthorizetokens").Name(name).Do().Error()
	return
}

func (c *oauthAuthorizeTokenInterface) Create(token *oauthapi.OAuthAuthorizeToken) (result *oauthapi.OAuthAuthorizeToken, err error) {
	result = &oauthapi.OAuthAuthorizeToken{}
	err = c.r.Post().Resource("oauthauthorizetokens").Body(token).Do().Into(result)
	return
}

func (c *oauthAuthorizeTokenInterface) Get(name string, options metav1.GetOptions) (result *oauthapi.OAuthAuthorizeToken, err error) {
	result = &oauthapi.OAuthAuthorizeToken{}
	err = c.r.Get().Resource("oauthauthorizetokens").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}
