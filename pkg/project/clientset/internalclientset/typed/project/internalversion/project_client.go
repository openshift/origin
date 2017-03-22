package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type ProjectInterface interface {
	RESTClient() restclient.Interface
	ProjectsGetter
}

// ProjectClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type ProjectClient struct {
	restClient restclient.Interface
}

func (c *ProjectClient) Projects() ProjectResourceInterface {
	return newProjects(c)
}

// NewForConfig creates a new ProjectClient for the given config.
func NewForConfig(c *restclient.Config) (*ProjectClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ProjectClient{client}, nil
}

// NewForConfigOrDie creates a new ProjectClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *ProjectClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ProjectClient for the given RESTClient.
func New(c restclient.Interface) *ProjectClient {
	return &ProjectClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if project group is not registered, return an error
	g, err := registered.Group("project.openshift.io")
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
func (c *ProjectClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
