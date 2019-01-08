package openshift_sdn

import (
	kclientset "k8s.io/client-go/kubernetes"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/network"
)

// informers is a small bag of data that holds our informers
type informers struct {
	KubeClient     kclientset.Interface
	NetworkClient  networkclient.Interface
	InternalClient kclientsetinternal.Interface

	// Internal kubernetes shared informer factory.
	InternalKubeInformers kinternalinformers.SharedInformerFactory
	// Network shared informer factory.
	NetworkInformers networkinformers.SharedInformerFactory
}

// buildInformers creates all the informer factories.
func (sdn *OpenShiftSDN) buildInformers() error {
	kubeConfig, err := configapi.GetKubeConfigOrInClusterConfig(sdn.NodeConfig.MasterKubeConfig, sdn.NodeConfig.MasterClientConnectionOverrides)
	if err != nil {
		return err
	}
	kubeClient, err := kclientset.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	internalClient, err := kclientsetinternal.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	networkClient, err := networkclient.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	internalKubeInformers := kinternalinformers.NewSharedInformerFactory(internalClient, sdn.ProxyConfig.IPTables.SyncPeriod.Duration)
	networkInformers := networkinformers.NewSharedInformerFactory(networkClient, network.DefaultInformerResyncPeriod)

	sdn.informers = &informers{
		KubeClient:     kubeClient,
		NetworkClient:  networkClient,
		InternalClient: internalClient,

		InternalKubeInformers: internalKubeInformers,
		NetworkInformers:      networkInformers,
	}
	return nil
}

// start starts the informers.
func (i *informers) start(stopCh <-chan struct{}) {
	i.InternalKubeInformers.Start(stopCh)
	i.NetworkInformers.Start(stopCh)
}
