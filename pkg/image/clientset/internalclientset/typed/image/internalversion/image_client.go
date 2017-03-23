package internalversion

import (
	api "k8s.io/kubernetes/pkg/api"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
)

type ImageInterface interface {
	RESTClient() restclient.Interface
	ImagesGetter
}

// ImageClient is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type ImageClient struct {
	restClient restclient.Interface
}

func (c *ImageClient) Images() ImageResourceInterface {
	return newImages(c)
}

// NewForConfig creates a new ImageClient for the given config.
func NewForConfig(c *restclient.Config) (*ImageClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ImageClient{client}, nil
}

// NewForConfigOrDie creates a new ImageClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *ImageClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ImageClient for the given RESTClient.
func New(c restclient.Interface) *ImageClient {
	return &ImageClient{c}
}

func setConfigDefaults(config *restclient.Config) error {
	// if image group is not registered, return an error
	g, err := registered.Group("image.openshift.io")
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
func (c *ImageClient) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
