package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type DeployInterface interface {
	RESTClient() restclient.Interface
	DeploymentConfigsGetter
}

// DeployClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type DeployClient struct {
	restClient restclient.Interface
}

func (c *DeployClient) DeploymentConfigs(namespace string) DeploymentConfigInterface {
	return newDeploymentConfigs(c, namespace)
}

// NewForConfig creates a new DeployClient for the given config.
func NewForConfig(c *restclient.Config) (*DeployClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &DeployClient{client}, nil
}

// NewForConfigOrDie creates a new DeployClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *DeployClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new DeployClient for the given RESTClient.
func New(c restclient.Interface) *DeployClient {
	return &DeployClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if deploy group is not registered, return an error
	g, err := registered.Group("apps.openshift.io")
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
func (c *DeployClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
