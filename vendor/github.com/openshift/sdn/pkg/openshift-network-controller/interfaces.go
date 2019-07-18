package openshift_network_controller

import (
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformer "github.com/openshift/client-go/network/informers/externalversions"
)

const defaultInformerResyncPeriod = 10 * time.Minute

func newControllerContext(clientConfig *rest.Config) (*ControllerContext, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	networkClient, err := networkclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	networkControllerContext := &ControllerContext{
		KubernetesClient:    kubeClient,
		KubernetesInformers: informers.NewSharedInformerFactory(kubeClient, defaultInformerResyncPeriod),
		NetworkClient:       networkClient,
		NetworkInformers:    networkinformer.NewSharedInformerFactory(networkClient, defaultInformerResyncPeriod),
	}

	return networkControllerContext, nil
}

type ControllerContext struct {
	KubernetesClient    kubernetes.Interface
	KubernetesInformers informers.SharedInformerFactory
	NetworkClient       networkclient.Interface
	NetworkInformers    networkinformer.SharedInformerFactory
}

func (c *ControllerContext) StartInformers() {
	c.KubernetesInformers.Start(nil)
	c.NetworkInformers.Start(nil)
}
