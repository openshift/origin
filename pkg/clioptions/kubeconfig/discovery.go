package kubeconfig

import (
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
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

type ConfigClientGetter struct {
	adminRESTConfig *rest.Config
}

func NewConfigClientGetter(adminRESTConfig *rest.Config) *ConfigClientGetter {
	return &ConfigClientGetter{
		adminRESTConfig: adminRESTConfig,
	}
}

func (d *ConfigClientGetter) GetConfigClient() (clientconfigv1.Interface, error) {
	ret, err := clientconfigv1.NewForConfig(d.adminRESTConfig)
	return ret, err
}
