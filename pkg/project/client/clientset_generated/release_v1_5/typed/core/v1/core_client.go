package v1

import (
	v1 "github.com/openshift/origin/pkg/project/api/v1"
	"github.com/openshift/origin/pkg/project/client/clientset_generated/release_v1_5/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type CoreV1Interface interface {
	RESTClient() rest.Interface
	ProjectsGetter
}

// CoreV1Client is used to interact with features provided by the  group.
type CoreV1Client struct {
	restClient rest.Interface
}

func (c *CoreV1Client) Projects() ProjectInterface {
	return newProjects(c)
}

// NewForConfig creates a new CoreV1Client for the given config.
func NewForConfig(c *rest.Config) (*CoreV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &CoreV1Client{client}, nil
}

// NewForConfigOrDie creates a new CoreV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *CoreV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreV1Client for the given RESTClient.
func New(c rest.Interface) *CoreV1Client {
	return &CoreV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/oapi"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
