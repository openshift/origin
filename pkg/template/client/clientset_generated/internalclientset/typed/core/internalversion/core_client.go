package internalversion

import (
	"github.com/openshift/origin/pkg/template/client/clientset_generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type CoreInterface interface {
	RESTClient() rest.Interface
	TemplatesGetter
}

// CoreClient is used to interact with features provided by the  group.
type CoreClient struct {
	restClient rest.Interface
}

func (c *CoreClient) Templates(namespace string) TemplateInterface {
	return newTemplates(c, namespace)
}

// NewForConfig creates a new CoreClient for the given config.
func NewForConfig(c *rest.Config) (*CoreClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &CoreClient{client}, nil
}

// NewForConfigOrDie creates a new CoreClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *CoreClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreClient for the given RESTClient.
func New(c rest.Interface) *CoreClient {
	return &CoreClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("")
	if err != nil {
		return err
	}

	config.APIPath = "/oapi"
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
func (c *CoreClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
