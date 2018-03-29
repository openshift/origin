package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type ControllerV1Interface interface {
	RESTClient() rest.Interface
	OpenShiftControllerConfigsGetter
}

// ControllerV1Client is used to interact with features provided by the controller group.
type ControllerV1Client struct {
	restClient rest.Interface
}

func (c *ControllerV1Client) OpenShiftControllerConfigs() OpenShiftControllerConfigInterface {
	return newOpenShiftControllerConfigs(c)
}

// NewForConfig creates a new ControllerV1Client for the given config.
func NewForConfig(c *rest.Config) (*ControllerV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ControllerV1Client{client}, nil
}

// NewForConfigOrDie creates a new ControllerV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ControllerV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ControllerV1Client for the given RESTClient.
func New(c rest.Interface) *ControllerV1Client {
	return &ControllerV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ControllerV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
