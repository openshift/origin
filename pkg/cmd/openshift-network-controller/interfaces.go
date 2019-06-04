package openshift_network_controller

import (
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformer "github.com/openshift/client-go/network/informers/externalversions"
)

func NewControllerContext(
	clientConfig *rest.Config,
	stopCh <-chan struct{},
) (*ControllerContext, error) {

	const defaultInformerResyncPeriod = 10 * time.Minute

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

		Stop:             stopCh,
		InformersStarted: make(chan struct{}),
	}

	return networkControllerContext, nil
}

type ControllerContext struct {
	KubernetesClient    kubernetes.Interface
	KubernetesInformers informers.SharedInformerFactory
	NetworkClient       networkclient.Interface
	NetworkInformers    networkinformer.SharedInformerFactory

	// Stop is the stop channel
	Stop <-chan struct{}

	informersStartedLock   sync.Mutex
	informersStartedClosed bool
	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}
}

func (c *ControllerContext) StartInformers(stopCh <-chan struct{}) {
	c.KubernetesInformers.Start(stopCh)
	c.NetworkInformers.Start(stopCh)

	c.informersStartedLock.Lock()
	defer c.informersStartedLock.Unlock()
	if !c.informersStartedClosed {
		close(c.InformersStarted)
		c.informersStartedClosed = true
	}
}
