package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type OAuthClientsInterface interface {
	OAuthClients() OAuthClientInterface
}

type OAuthClientInterface interface {
	Create(obj *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error)
	List(opts metav1.ListOptions) (*oauthapi.OAuthClientList, error)
	Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClient, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Update(client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error)
}

type oauthClients struct {
	r *Client
}

func newOAuthClients(c *Client) *oauthClients {
	return &oauthClients{
		r: c,
	}
}

func (c *oauthClients) Create(obj *oauthapi.OAuthClient) (result *oauthapi.OAuthClient, err error) {
	result = &oauthapi.OAuthClient{}
	err = c.r.Post().Resource("oauthclients").Body(obj).Do().Into(result)
	return
}

func (c *oauthClients) List(opts metav1.ListOptions) (result *oauthapi.OAuthClientList, err error) {
	result = &oauthapi.OAuthClientList{}
	err = c.r.Get().Resource("oauthclients").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c *oauthClients) Get(name string, options metav1.GetOptions) (result *oauthapi.OAuthClient, err error) {
	result = &oauthapi.OAuthClient{}
	err = c.r.Get().Resource("oauthclients").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c *oauthClients) Delete(name string) (err error) {
	err = c.r.Delete().Resource("oauthclients").Name(name).Do().Error()
	return
}

func (c *oauthClients) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("oauthclients").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}

func (c *oauthClients) Update(client *oauthapi.OAuthClient) (result *oauthapi.OAuthClient, err error) {
	result = &oauthapi.OAuthClient{}
	err = c.r.Put().Resource("oauthclients").Name(client.Name).Body(client).Do().Into(result)
	return
}
