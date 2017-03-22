package v1

import (
	fmt "fmt"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	serializer "k8s.io/kubernetes/pkg/runtime/serializer"
)

type BuildV1Interface interface {
	RESTClient() restclient.Interface
	BuildsGetter
}

// BuildV1Client is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type BuildV1Client struct {
	restClient restclient.Interface
}

func (c *BuildV1Client) Builds(namespace string) BuildResourceInterface {
	return newBuilds(c, namespace)
}

// NewForConfig creates a new BuildV1Client for the given config.
func NewForConfig(c *restclient.Config) (*BuildV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &BuildV1Client{client}, nil
}

// NewForConfigOrDie creates a new BuildV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *BuildV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new BuildV1Client for the given RESTClient.
func New(c restclient.Interface) *BuildV1Client {
	return &BuildV1Client{c}
}

func setConfigDefaults(config *restclient.Config) error {
	gv, err := unversioned.ParseGroupVersion("build.openshift.io/v1")
	if err != nil {
		return err
	}
	// if build.openshift.io/v1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("build.openshift.io/v1 is not enabled")
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
func (c *BuildV1Client) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
