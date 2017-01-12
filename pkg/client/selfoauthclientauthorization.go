package client

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type SelfOAuthClientAuthorizationsInterface interface {
	SelfOAuthClientAuthorizations() SelfOAuthClientAuthorizationInterface
}

type SelfOAuthClientAuthorizationInterface interface {
	List(opts kapi.ListOptions) (*oauthapi.SelfOAuthClientAuthorizationList, error)
	Get(name string) (*oauthapi.SelfOAuthClientAuthorization, error)
	Delete(name string) error
	Watch(opts kapi.ListOptions) (watch.Interface, error)
}

type selfOAuthClientAuthorizations struct {
	r *Client
}

func newSelfOAuthClientAuthorizations(c *Client) *selfOAuthClientAuthorizations {
	return &selfOAuthClientAuthorizations{
		r: c,
	}
}

func (c *selfOAuthClientAuthorizations) List(opts kapi.ListOptions) (result *oauthapi.SelfOAuthClientAuthorizationList, err error) {
	result = &oauthapi.SelfOAuthClientAuthorizationList{}
	err = c.r.Get().Resource("selfoauthclientauthorizations").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c *selfOAuthClientAuthorizations) Get(name string) (result *oauthapi.SelfOAuthClientAuthorization, err error) {
	result = &oauthapi.SelfOAuthClientAuthorization{}
	err = c.r.Get().Resource("selfoauthclientauthorizations").Name(name).Do().Into(result)
	return
}

func (c *selfOAuthClientAuthorizations) Delete(name string) (err error) {
	err = c.r.Delete().Resource("selfoauthclientauthorizations").Name(name).Do().Error()
	return
}

func (c *selfOAuthClientAuthorizations) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("selfoauthclientauthorizations").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
