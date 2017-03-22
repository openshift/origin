package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type TemplateInterface interface {
	RESTClient() restclient.Interface
	BrokerTemplateInstancesGetter
	TemplatesGetter
	TemplateInstancesGetter
}

// TemplateClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type TemplateClient struct {
	restClient restclient.Interface
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
func NewForConfig(c *restclient.Config) (*TemplateClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &TemplateClient{client}, nil
}

// NewForConfigOrDie creates a new TemplateClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *TemplateClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new TemplateClient for the given RESTClient.
func New(c restclient.Interface) *TemplateClient {
	return &TemplateClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if template group is not registered, return an error
	g, err := registered.Group("template.openshift.io")
	if err != nil {
		return err
	}
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = restclient.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		copyGroupVersion := g.GroupVersion
		config.GroupVersion = &copyGroupVersion
	}
	config.NegotiatedSerializer = api.Codecs

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
func (c *TemplateClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
