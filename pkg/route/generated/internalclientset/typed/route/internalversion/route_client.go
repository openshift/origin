package internalversion

import (
	"github.com/openshift/origin/pkg/route/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type RouteInterface interface {
	RESTClient() rest.Interface
	RoutesGetter
}

// RouteClient is used to interact with features provided by the route.openshift.io group.
type RouteClient struct {
	restClient rest.Interface
}

func (c *RouteClient) Routes(namespace string) RouteResourceInterface {
	return newRoutes(c, namespace)
}

// NewForConfig creates a new RouteClient for the given config.
func NewForConfig(c *rest.Config) (*RouteClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &RouteClient{client}, nil
}

// NewForConfigOrDie creates a new RouteClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *RouteClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new RouteClient for the given RESTClient.
func New(c rest.Interface) *RouteClient {
	return &RouteClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("route.openshift.io")
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
func (c *RouteClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
