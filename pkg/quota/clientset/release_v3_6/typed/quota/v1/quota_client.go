package v1

import (
	fmt "fmt"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	registered "k8s.io/kubernetes/pkg/apimachinery/registered"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	serializer "k8s.io/kubernetes/pkg/runtime/serializer"
)

type QuotaV1Interface interface {
	RESTClient() restclient.Interface
	ClusterResourceQuotasGetter
}

// QuotaV1Client is used to interact with features provided by the k8s.io/kubernetes/pkg/apimachinery/registered.Group group.
type QuotaV1Client struct {
	restClient restclient.Interface
}

func (c *QuotaV1Client) ClusterResourceQuotas() ClusterResourceQuotaInterface {
	return newClusterResourceQuotas(c)
}

// NewForConfig creates a new QuotaV1Client for the given config.
func NewForConfig(c *restclient.Config) (*QuotaV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &QuotaV1Client{client}, nil
}

// NewForConfigOrDie creates a new QuotaV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *QuotaV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new QuotaV1Client for the given RESTClient.
func New(c restclient.Interface) *QuotaV1Client {
	return &QuotaV1Client{c}
}

func setConfigDefaults(config *restclient.Config) error {
	gv, err := unversioned.ParseGroupVersion("quota.openshift.io/v1")
	if err != nil {
		return err
	}
	// if quota.openshift.io/v1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("quota.openshift.io/v1 is not enabled")
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
func (c *QuotaV1Client) RESTClient() restclient.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
