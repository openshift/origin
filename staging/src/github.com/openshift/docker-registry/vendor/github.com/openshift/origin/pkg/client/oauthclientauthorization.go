package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type OAuthClientAuthorizationsInterface interface {
	OAuthClientAuthorizations() OAuthClientAuthorizationInterface
}

type OAuthClientAuthorizationInterface interface {
	Create(obj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error)
	List(opts metav1.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error)
	Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error)
	Update(obj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type oauthClientAuthorizations struct {
	r *Client
}

func newOAuthClientAuthorizations(c *Client) *oauthClientAuthorizations {
	return &oauthClientAuthorizations{
		r: c,
	}
}

func (c *oauthClientAuthorizations) Create(obj *oauthapi.OAuthClientAuthorization) (result *oauthapi.OAuthClientAuthorization, err error) {
	result = &oauthapi.OAuthClientAuthorization{}
	err = c.r.Post().Resource("oauthclientauthorizations").Body(obj).Do().Into(result)
	return
}

func (c *oauthClientAuthorizations) Update(obj *oauthapi.OAuthClientAuthorization) (result *oauthapi.OAuthClientAuthorization, err error) {
	result = &oauthapi.OAuthClientAuthorization{}
	err = c.r.Put().Resource("oauthclientauthorizations").Name(obj.Name).Body(obj).Do().Into(result)
	return
}

func (c *oauthClientAuthorizations) List(opts metav1.ListOptions) (result *oauthapi.OAuthClientAuthorizationList, err error) {
	result = &oauthapi.OAuthClientAuthorizationList{}
	err = c.r.Get().Resource("oauthclientauthorizations").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c *oauthClientAuthorizations) Get(name string, options metav1.GetOptions) (result *oauthapi.OAuthClientAuthorization, err error) {
	result = &oauthapi.OAuthClientAuthorization{}
	err = c.r.Get().Resource("oauthclientauthorizations").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c *oauthClientAuthorizations) Delete(name string) (err error) {
	err = c.r.Delete().Resource("oauthclientauthorizations").Name(name).Do().Error()
	return
}

func (c *oauthClientAuthorizations) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("oauthclientauthorizations").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
