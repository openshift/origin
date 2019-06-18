package openshift_sdn

import (
	"net"
	"net/http"
	"time"

	kinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
)

var defaultInformerResyncPeriod = 30 * time.Minute

// informers is a small bag of data that holds our informers
type informers struct {
	KubeClient    kubernetes.Interface
	NetworkClient networkclient.Interface

	// External kubernetes shared informer factory.
	KubeInformers kinformers.SharedInformerFactory
	// Network shared informer factory.
	NetworkInformers networkinformers.SharedInformerFactory
}

// buildInformers creates all the informer factories.
func (sdn *OpenShiftSDN) buildInformers() error {
	kubeConfig, err := getInClusterConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	networkClient, err := networkclient.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	kubeInformers := kinformers.NewSharedInformerFactory(kubeClient, sdn.ProxyConfig.IPTables.SyncPeriod.Duration)
	networkInformers := networkinformers.NewSharedInformerFactory(networkClient, defaultInformerResyncPeriod)

	sdn.informers = &informers{
		KubeClient:    kubeClient,
		NetworkClient: networkClient,

		KubeInformers:    kubeInformers,
		NetworkInformers: networkInformers,
	}
	return nil
}

// start starts the informers.
func (i *informers) start(stopCh <-chan struct{}) {
	i.KubeInformers.Start(stopCh)
	i.NetworkInformers.Start(stopCh)
}

// getInClusterConfig loads in-cluster config, then applies default overrides.
func getInClusterConfig() (*rest.Config, error) {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

// defaultClientTransport sets defaults for a client Transport that are suitable
// for use by infrastructure components.
func defaultClientTransport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	if !ok {
		return rt
	}

	// TODO: this should be configured by the caller, not in this method.
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport.Dial = dialer.Dial
	// Hold open more internal idle connections
	// TODO: this should be configured by the caller, not in this method.
	transport.MaxIdleConnsPerHost = 100
	return transport
}

// applyClientConnectionOverrides updates a kubeConfig with default overrides
func applyClientConnectionOverrides(kubeConfig *rest.Config) {
	kubeConfig.QPS = 10.0
	kubeConfig.Burst = 20
}
