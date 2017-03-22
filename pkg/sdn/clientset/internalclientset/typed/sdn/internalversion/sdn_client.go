package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type SdnInterface interface {
	RESTClient() restclient.Interface
	ClusterNetworksGetter
}

// SdnClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type SdnClient struct {
	restClient restclient.Interface
}

func (c *SdnClient) ClusterNetworks(namespace string) ClusterNetworkInterface {
	return newClusterNetworks(c, namespace)
}

// NewForConfig creates a new SdnClient for the given config.
func NewForConfig(c *restclient.Config) (*SdnClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SdnClient{client}, nil
}

// NewForConfigOrDie creates a new SdnClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *SdnClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SdnClient for the given RESTClient.
func New(c restclient.Interface) *SdnClient {
	return &SdnClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if sdn group is not registered, return an error
	g, err := registered.Group("network.openshift.io")
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
func (c *SdnClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
