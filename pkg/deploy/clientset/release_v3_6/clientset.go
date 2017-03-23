package release_v3_6

import (
	"github.com/golang/glog"
	v1deploy "github.com/openshift/origin/pkg/deploy/clientset/release_v3_6/typed/deploy/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	discovery "k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	_ "k8s.io/kubernetes/plugin/pkg/client/auth"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	DeployV1() v1deploy.DeployV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Deploy() v1deploy.DeployV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	*v1deploy.DeployV1Client
}

// DeployV1 retrieves the DeployV1Client
func (c *Clientset) DeployV1() v1deploy.DeployV1Interface {
	if c == nil {
		return nil
	}
	return c.DeployV1Client
}

// Deprecated: Deploy retrieves the default version of DeployClient.
// Please explicitly pick a version.
func (c *Clientset) Deploy() v1deploy.DeployV1Interface {
	if c == nil {
		return nil
	}
	return c.DeployV1Client
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
	clientset.DeployV1Client, err = v1deploy.NewForConfig(&configShallowCopy)
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
	clientset.DeployV1Client = v1deploy.NewForConfigOrDie(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &clientset
}

// New creates a new Clientset for the given RESTClient.
func New(c restclient.Interface) *Clientset {
	var clientset Clientset
	clientset.DeployV1Client = v1deploy.New(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &clientset
}
