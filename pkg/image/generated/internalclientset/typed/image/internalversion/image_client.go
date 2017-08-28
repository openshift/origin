package internalversion

import (
	"github.com/openshift/origin/pkg/image/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type ImageInterface interface {
	RESTClient() rest.Interface
	ImagesGetter
	ImageSignaturesGetter
	ImageStreamsGetter
	ImageStreamImagesGetter
	ImageStreamImportsGetter
	ImageStreamMappingsGetter
	ImageStreamTagsGetter
}

// ImageClient is used to interact with features provided by the image.openshift.io group.
type ImageClient struct {
	restClient rest.Interface
}

func (c *ImageClient) Images() ImageResourceInterface {
	return newImages(c)
}

func (c *ImageClient) ImageSignatures() ImageSignatureInterface {
	return newImageSignatures(c)
}

func (c *ImageClient) ImageStreams(namespace string) ImageStreamInterface {
	return newImageStreams(c, namespace)
}

func (c *ImageClient) ImageStreamImages(namespace string) ImageStreamImageInterface {
	return newImageStreamImages(c, namespace)
}

func (c *ImageClient) ImageStreamImports(namespace string) ImageStreamImportInterface {
	return newImageStreamImports(c, namespace)
}

func (c *ImageClient) ImageStreamMappings(namespace string) ImageStreamMappingInterface {
	return newImageStreamMappings(c, namespace)
}

func (c *ImageClient) ImageStreamTags(namespace string) ImageStreamTagInterface {
	return newImageStreamTags(c, namespace)
}

// NewForConfig creates a new ImageClient for the given config.
func NewForConfig(c *rest.Config) (*ImageClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ImageClient{client}, nil
}

// NewForConfigOrDie creates a new ImageClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ImageClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ImageClient for the given RESTClient.
func New(c rest.Interface) *ImageClient {
	return &ImageClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("image.openshift.io")
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
func (c *ImageClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
