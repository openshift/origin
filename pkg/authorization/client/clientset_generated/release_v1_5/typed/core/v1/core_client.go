package v1

import (
	fmt "fmt"

	registered "k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	restclient "k8s.io/client-go/rest"
	api "k8s.io/kubernetes/pkg/api"
)

type CoreV1Interface interface {
	RESTClient() restclient.Interface
	PoliciesGetter
}

// CoreV1Client is used to interact with features provided by the k8s.io/apimachinery/pkg/apimachinery/registered.Group group.
type CoreV1Client struct {
	restClient restclient.Interface
}

func (c *CoreV1Client) Policies(namespace string) PolicyInterface {
	return newPolicies(c, namespace)
}

// NewForConfig creates a new CoreV1Client for the given config.
func NewForConfig(c *restclient.Config) (*CoreV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &CoreV1Client{client}, nil
}

// NewForConfigOrDie creates a new CoreV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *CoreV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreV1Client for the given RESTClient.
func New(c restclient.Interface) *CoreV1Client {
	return &CoreV1Client{c}
}

func setConfigDefaults(config *restclient.Config) error {
	gv, err := schema.ParseGroupVersion("/v1")
	if err != nil {
		return err
	}
	// if /v1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("/v1 is not enabled")
	}
	config.APIPath = "/oapi"
	if config.UserAgent == "" {
		config.UserAgent = restclient.DefaultKubernetesUserAgent()
	}
	copyGroupVersion := gv
	config.GroupVersion = &copyGroupVersion

	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1Client) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
