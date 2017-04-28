package internalversion

import (
	"github.com/openshift/origin/pkg/sdn/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type SdnInterface interface {
	RESTClient() rest.Interface
	ClusterNetworksGetter
}

// SdnClient is used to interact with features provided by the network.openshift.io group.
type SdnClient struct {
	restClient rest.Interface
}

func (c *SdnClient) ClusterNetworks(namespace string) ClusterNetworkInterface {
	return newClusterNetworks(c, namespace)
}

// NewForConfig creates a new SdnClient for the given config.
func NewForConfig(c *rest.Config) (*SdnClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SdnClient{client}, nil
}

// NewForConfigOrDie creates a new SdnClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SdnClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SdnClient for the given RESTClient.
func New(c rest.Interface) *SdnClient {
	return &SdnClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("network.openshift.io")
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
func (c *SdnClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
