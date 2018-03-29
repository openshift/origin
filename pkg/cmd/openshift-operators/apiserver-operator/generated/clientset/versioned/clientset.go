package versioned

import (
	glog "github.com/golang/glog"
	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/typed/apiserver/v1"
	discovery "k8s.io/client-go/discovery"
	rest "k8s.io/client-go/rest"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	ApiserverV1() apiserverv1.ApiserverV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Apiserver() apiserverv1.ApiserverV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	apiserverV1 *apiserverv1.ApiserverV1Client
}

// ApiserverV1 retrieves the ApiserverV1Client
func (c *Clientset) ApiserverV1() apiserverv1.ApiserverV1Interface {
	return c.apiserverV1
}

// Deprecated: Apiserver retrieves the default version of ApiserverClient.
// Please explicitly pick a version.
func (c *Clientset) Apiserver() apiserverv1.ApiserverV1Interface {
	return c.apiserverV1
}

// Discovery retrieves the DiscoveryClient
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	if c == nil {
		return nil
	}
	return c.DiscoveryClient
}

// NewForConfig creates a new Clientset for the given config.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error
	cs.apiserverV1, err = apiserverv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	cs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(&configShallowCopy)
	if err != nil {
		glog.Errorf("failed to create the DiscoveryClient: %v", err)
		return nil, err
	}
	return &cs, nil
}

// NewForConfigOrDie creates a new Clientset for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *Clientset {
	var cs Clientset
	cs.apiserverV1 = apiserverv1.NewForConfigOrDie(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &cs
}

// New creates a new Clientset for the given RESTClient.
func New(c rest.Interface) *Clientset {
	var cs Clientset
	cs.apiserverV1 = apiserverv1.New(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &cs
}
