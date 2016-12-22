package v1

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	serializer "k8s.io/kubernetes/pkg/runtime/serializer"
)

type CoreInterface interface {
	GetRESTClient() *restclient.RESTClient
	ImagesGetter
}

// CoreClient is used to interact with features provided by the Core group.
type CoreClient struct {
	*restclient.RESTClient
}

func (c *CoreClient) Images(namespace string) ImageInterface {
	return newImages(c, namespace)
}

// NewForConfig creates a new CoreClient for the given config.
func NewForConfig(c *restclient.Config) (*CoreClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &CoreClient{client}, nil
}

// NewForConfigOrDie creates a new CoreClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *CoreClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreClient for the given RESTClient.
func New(c *restclient.RESTClient) *CoreClient {
	return &CoreClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if core group is not registered, return an error
	g, err := registered.Group("")
	if err != nil {
		return err
	}
	config.APIPath = "/oapi"
	if config.UserAgent == "" {
		config.UserAgent = restclient.DefaultKubernetesUserAgent()
	}
	// TODO: Unconditionally set the config.Version, until we fix the config.
	//if config.Version == "" {
	copyGroupVersion := g.GroupVersion
	config.GroupVersion = &copyGroupVersion
	//}

	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	return nil
}

// GetRESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreClient) GetRESTClient() *restclient.RESTClient {
	if c == nil {
		return nil
	}
	return c.RESTClient
}
