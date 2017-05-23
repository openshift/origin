package internalversion

import (
	"github.com/openshift/origin/pkg/build/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type BuildInterface interface {
	RESTClient() rest.Interface
	BuildsGetter
	BuildConfigsGetter
}

// BuildClient is used to interact with features provided by the build.openshift.io group.
type BuildClient struct {
	restClient rest.Interface
}

func (c *BuildClient) Builds(namespace string) BuildResourceInterface {
	return newBuilds(c, namespace)
}

func (c *BuildClient) BuildConfigs(namespace string) BuildConfigInterface {
	return newBuildConfigs(c, namespace)
}

// NewForConfig creates a new BuildClient for the given config.
func NewForConfig(c *rest.Config) (*BuildClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &BuildClient{client}, nil
}

// NewForConfigOrDie creates a new BuildClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *BuildClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new BuildClient for the given RESTClient.
func New(c rest.Interface) *BuildClient {
	return &BuildClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("build.openshift.io")
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
func (c *BuildClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
