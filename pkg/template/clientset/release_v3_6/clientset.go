package release_v3_6

import (
	"github.com/golang/glog"
	v1template "github.com/openshift/origin/pkg/template/clientset/release_v3_6/typed/template/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	discovery "k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	_ "k8s.io/kubernetes/plugin/pkg/client/auth"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	TemplateV1() v1template.TemplateV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Template() v1template.TemplateV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	*v1template.TemplateV1Client
}

// TemplateV1 retrieves the TemplateV1Client
func (c *Clientset) TemplateV1() v1template.TemplateV1Interface {
	if c == nil {
		return nil
	}
	return c.TemplateV1Client
}

// Deprecated: Template retrieves the default version of TemplateClient.
// Please explicitly pick a version.
func (c *Clientset) Template() v1template.TemplateV1Interface {
	if c == nil {
		return nil
	}
	return c.TemplateV1Client
}

// Discovery retrieves the DiscoveryClient
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.DiscoveryClient
}

// NewForConfig creates a new Clientset for the given config.
func NewForConfig(c *restclient.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var clientset Clientset
	var err error
	clientset.TemplateV1Client, err = v1template.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	clientset.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(&configShallowCopy)
	if err != nil {
		glog.Errorf("failed to create the DiscoveryClient: %v", err)
		return nil, err
	}
	return &clientset, nil
}

// NewForConfigOrDie creates a new Clientset for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *restclient.Config) *Clientset {
	var clientset Clientset
	clientset.TemplateV1Client = v1template.NewForConfigOrDie(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &clientset
}

// New creates a new Clientset for the given RESTClient.
func New(c restclient.Interface) *Clientset {
	var clientset Clientset
	clientset.TemplateV1Client = v1template.New(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &clientset
}
