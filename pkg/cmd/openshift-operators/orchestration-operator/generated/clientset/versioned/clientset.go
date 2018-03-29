package versioned

import (
	glog "github.com/golang/glog"
	orchestrationv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/generated/clientset/versioned/typed/orchestration/v1"
	discovery "k8s.io/client-go/discovery"
	rest "k8s.io/client-go/rest"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	OrchestrationV1() orchestrationv1.OrchestrationV1Interface
	// Deprecated: please explicitly pick a version if possible.
	Orchestration() orchestrationv1.OrchestrationV1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	orchestrationV1 *orchestrationv1.OrchestrationV1Client
}

// OrchestrationV1 retrieves the OrchestrationV1Client
func (c *Clientset) OrchestrationV1() orchestrationv1.OrchestrationV1Interface {
	return c.orchestrationV1
}

// Deprecated: Orchestration retrieves the default version of OrchestrationClient.
// Please explicitly pick a version.
func (c *Clientset) Orchestration() orchestrationv1.OrchestrationV1Interface {
	return c.orchestrationV1
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
	cs.orchestrationV1, err = orchestrationv1.NewForConfig(&configShallowCopy)
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
	cs.orchestrationV1 = orchestrationv1.NewForConfigOrDie(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &cs
}

// New creates a new Clientset for the given RESTClient.
func New(c rest.Interface) *Clientset {
	var cs Clientset
	cs.orchestrationV1 = orchestrationv1.New(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &cs
}
