package openshift_network_controller

import (
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformer "github.com/openshift/client-go/network/informers/externalversions"
)

func NewControllerContext(
	config openshiftcontrolplanev1.OpenShiftControllerManagerConfig,
	inClientConfig *rest.Config,
	stopCh <-chan struct{},
) (*ControllerContext, error) {

	const defaultInformerResyncPeriod = 10 * time.Minute

	// copy to avoid messing with original
	clientConfig := rest.CopyConfig(inClientConfig)
	// divide up the QPS since it re-used separately for every client
	// TODO, eventually make this configurable individually in some way.
	if clientConfig.QPS > 0 {
		clientConfig.QPS = clientConfig.QPS/10 + 1
	}
	if clientConfig.Burst > 0 {
		clientConfig.Burst = clientConfig.Burst/10 + 1
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	networkClient, err := networkclient.NewForConfig(nonProtobufConfig(clientConfig))
	if err != nil {
		return nil, err
	}

	networkControllerContext := &ControllerContext{
		OpenshiftControllerConfig: config,

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
	OpenshiftControllerConfig openshiftcontrolplanev1.OpenShiftControllerManagerConfig

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

// nonProtobufConfig returns a copy of inConfig that doesn't force the use of protobufs,
// for working with CRD-based APIs.
func nonProtobufConfig(inConfig *rest.Config) *rest.Config {
	npConfig := rest.CopyConfig(inConfig)
	npConfig.ContentConfig.AcceptContentTypes = "application/json"
	npConfig.ContentConfig.ContentType = "application/json"
	return npConfig
}
