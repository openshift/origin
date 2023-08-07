package kubeconfig

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type DiscoveryGetter struct {
	adminRESTConfig *rest.Config
}

func NewDiscoveryGetter(adminRESTConfig *rest.Config) *DiscoveryGetter {
	return &DiscoveryGetter{
		adminRESTConfig: adminRESTConfig,
	}
}

func (d *DiscoveryGetter) GetDiscoveryClient() (discovery.AggregatedDiscoveryInterface, error) {
	ret, err := discovery.NewDiscoveryClientForConfig(d.adminRESTConfig)
	return ret, err
}
