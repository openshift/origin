package v1

import (
	fmt "fmt"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	serializer "k8s.io/kubernetes/pkg/runtime/serializer"
)

type RouteV1Interface interface {
	RESTClient() restclient.Interface
	RoutesGetter
}

// RouteV1Client is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type RouteV1Client struct {
	restClient restclient.Interface
}

func (c *RouteV1Client) Routes(namespace string) RouteResourceInterface {
	return newRoutes(c, namespace)
}

// NewForConfig creates a new RouteV1Client for the given config.
func NewForConfig(c *restclient.Config) (*RouteV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &RouteV1Client{client}, nil
}

// NewForConfigOrDie creates a new RouteV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *RouteV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new RouteV1Client for the given RESTClient.
func New(c restclient.Interface) *RouteV1Client {
	return &RouteV1Client{c}
}

func setConfigDefaults(config *restclient.Config) error {
	gv, err := unversioned.ParseGroupVersion("route.openshift.io/v1")
	if err != nil {
		return err
	}
	// if route.openshift.io/v1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("route.openshift.io/v1 is not enabled")
	}
	config.APIPath = "/apis"
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
func (c *RouteV1Client) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
