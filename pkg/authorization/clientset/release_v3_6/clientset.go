package release_v3_6

import (
	"github.com/golang/glog"
	v1authorization "github.com/openshift/origin/pkg/authorization/clientset/release_v3_6/typed/authorization/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	discovery "k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	_ "k8s.io/kubernetes/plugin/pkg/client/auth"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	AuthorizationV1() v1authorization.AuthorizationV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Authorization() v1authorization.AuthorizationV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	*v1authorization.AuthorizationV1Client
}

// AuthorizationV1 retrieves the AuthorizationV1Client
func (c *Clientset) AuthorizationV1() v1authorization.AuthorizationV1Interface {
	if c == nil {
		return nil
	}
	return c.AuthorizationV1Client
}

// Deprecated: Authorization retrieves the default version of AuthorizationClient.
// Please explicitly pick a version.
func (c *Clientset) Authorization() v1authorization.AuthorizationV1Interface {
	if c == nil {
		return nil
	}
	return c.AuthorizationV1Client
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
	clientset.AuthorizationV1Client, err = v1authorization.NewForConfig(&configShallowCopy)
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
	clientset.AuthorizationV1Client = v1authorization.NewForConfigOrDie(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &clientset
}

// New creates a new Clientset for the given RESTClient.
func New(c restclient.Interface) *Clientset {
	var clientset Clientset
	clientset.AuthorizationV1Client = v1authorization.New(c)

	clientset.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &clientset
}
