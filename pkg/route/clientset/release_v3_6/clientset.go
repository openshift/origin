package release_v3_6

import (
	"github.com/golang/glog"
	v1route "github.com/openshift/origin/pkg/route/clientset/release_v3_6/typed/route/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	discovery "k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	_ "k8s.io/kubernetes/plugin/pkg/client/auth"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	RouteV1() v1route.RouteV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Route() v1route.RouteV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	*v1route.RouteV1Client
}

// RouteV1 retrieves the RouteV1Client
func (c *Clientset) RouteV1() v1route.RouteV1Interface {
	if c == nil {
		return nil
	}
	return c.RouteV1Client
}

// Deprecated: Route retrieves the default version of RouteClient.
// Please explicitly pick a version.
func (c *Clientset) Route() v1route.RouteV1Interface {
	if c == nil {
		return nil
	}
	return c.RouteV1Client
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
	clientset.RouteV1Client, err = v1route.NewForConfig(&configShallowCopy)
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
	clientset.RouteV1Client = v1route.NewForConfigOrDie(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &clientset
}

// New creates a new Clientset for the given RESTClient.
func New(c restclient.Interface) *Clientset {
	var clientset Clientset
	clientset.RouteV1Client = v1route.New(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &clientset
}
