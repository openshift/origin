package internalversion

import (
	"github.com/openshift/origin/pkg/template/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type TemplateInterface interface {
	RESTClient() rest.Interface
	BrokerTemplateInstancesGetter
	TemplatesGetter
	TemplateInstancesGetter
}

// TemplateClient is used to interact with features provided by the template.openshift.io group.
type TemplateClient struct {
	restClient rest.Interface
}

func (c *TemplateClient) BrokerTemplateInstances() BrokerTemplateInstanceInterface {
	return newBrokerTemplateInstances(c)
}

func (c *TemplateClient) Templates(namespace string) TemplateResourceInterface {
	return newTemplates(c, namespace)
}

func (c *TemplateClient) TemplateInstances(namespace string) TemplateInstanceInterface {
	return newTemplateInstances(c, namespace)
}

// NewForConfig creates a new TemplateClient for the given config.
func NewForConfig(c *rest.Config) (*TemplateClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &TemplateClient{client}, nil
}

// NewForConfigOrDie creates a new TemplateClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *TemplateClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new TemplateClient for the given RESTClient.
func New(c rest.Interface) *TemplateClient {
	return &TemplateClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("template.openshift.io")
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
func (c *TemplateClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
