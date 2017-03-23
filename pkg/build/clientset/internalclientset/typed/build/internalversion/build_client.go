package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type BuildInterface interface {
	RESTClient() restclient.Interface
	BuildsGetter
}

// BuildClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type BuildClient struct {
	restClient restclient.Interface
}

func (c *BuildClient) Builds(namespace string) BuildResourceInterface {
	return newBuilds(c, namespace)
}

// NewForConfig creates a new BuildClient for the given config.
func NewForConfig(c *restclient.Config) (*BuildClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &BuildClient{client}, nil
}

// NewForConfigOrDie creates a new BuildClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *BuildClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new BuildClient for the given RESTClient.
func New(c restclient.Interface) *BuildClient {
	return &BuildClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if build group is not registered, return an error
	g, err := registered.Group("build.openshift.io")
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
func (c *BuildClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
