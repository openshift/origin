package internalversion

import (
	"github.com/openshift/origin/pkg/project/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type ProjectInterface interface {
	RESTClient() rest.Interface
	ProjectsGetter
	ProjectRequestsGetter
}

// ProjectClient is used to interact with features provided by the project.openshift.io group.
type ProjectClient struct {
	restClient rest.Interface
}

func (c *ProjectClient) Projects() ProjectResourceInterface {
	return newProjects(c)
}

func (c *ProjectClient) ProjectRequests() ProjectRequestInterface {
	return newProjectRequests(c)
}

// NewForConfig creates a new ProjectClient for the given config.
func NewForConfig(c *rest.Config) (*ProjectClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ProjectClient{client}, nil
}

// NewForConfigOrDie creates a new ProjectClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ProjectClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ProjectClient for the given RESTClient.
func New(c rest.Interface) *ProjectClient {
	return &ProjectClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("project.openshift.io")
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
func (c *ProjectClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
