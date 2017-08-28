package internalversion

import (
	"github.com/openshift/origin/pkg/oauth/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type OauthInterface interface {
	RESTClient() rest.Interface
	OAuthAccessTokensGetter
	OAuthAuthorizeTokensGetter
	OAuthClientsGetter
	OAuthClientAuthorizationsGetter
}

// OauthClient is used to interact with features provided by the oauth.openshift.io group.
type OauthClient struct {
	restClient rest.Interface
}

func (c *OauthClient) OAuthAccessTokens() OAuthAccessTokenInterface {
	return newOAuthAccessTokens(c)
}

func (c *OauthClient) OAuthAuthorizeTokens() OAuthAuthorizeTokenInterface {
	return newOAuthAuthorizeTokens(c)
}

func (c *OauthClient) OAuthClients() OAuthClientInterface {
	return newOAuthClients(c)
}

func (c *OauthClient) OAuthClientAuthorizations() OAuthClientAuthorizationInterface {
	return newOAuthClientAuthorizations(c)
}

// NewForConfig creates a new OauthClient for the given config.
func NewForConfig(c *rest.Config) (*OauthClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &OauthClient{client}, nil
}

// NewForConfigOrDie creates a new OauthClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *OauthClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new OauthClient for the given RESTClient.
func New(c rest.Interface) *OauthClient {
	return &OauthClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("oauth.openshift.io")
	if err != nil {
		return err
	}

	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *OauthClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
