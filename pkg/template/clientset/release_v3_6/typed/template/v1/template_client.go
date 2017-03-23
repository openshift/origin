package v1

import (
	fmt "fmt"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	serializer "k8s.io/kubernetes/pkg/runtime/serializer"
)

type TemplateV1Interface interface {
	RESTClient() restclient.Interface
	BrokerTemplateInstancesGetter
	TemplatesGetter
	TemplateInstancesGetter
}

// TemplateV1Client is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type TemplateV1Client struct {
	restClient restclient.Interface
}

func (c *TemplateV1Client) BrokerTemplateInstances() BrokerTemplateInstanceInterface {
	return newBrokerTemplateInstances(c)
}

func (c *TemplateV1Client) Templates(namespace string) TemplateResourceInterface {
	return newTemplates(c, namespace)
}

func (c *TemplateV1Client) TemplateInstances(namespace string) TemplateInstanceInterface {
	return newTemplateInstances(c, namespace)
}

// NewForConfig creates a new TemplateV1Client for the given config.
func NewForConfig(c *restclient.Config) (*TemplateV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &TemplateV1Client{client}, nil
}

// NewForConfigOrDie creates a new TemplateV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *TemplateV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new TemplateV1Client for the given RESTClient.
func New(c restclient.Interface) *TemplateV1Client {
	return &TemplateV1Client{c}
}

func setConfigDefaults(config *restclient.Config) error {
	gv, err := unversioned.ParseGroupVersion("template.openshift.io/v1")
	if err != nil {
		return err
	}
	// if template.openshift.io/v1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("template.openshift.io/v1 is not enabled")
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
func (c *TemplateV1Client) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
