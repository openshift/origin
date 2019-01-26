package openshift_sdn

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/network"
)

// informers is a small bag of data that holds our informers
type informers struct {
	KubeClient     kubernetes.Interface
	NetworkClient  networkclient.Interface
	InternalClient kclientsetinternal.Interface

	// External kubernetes shared informer factory.
	KubeInformers kinformers.SharedInformerFactory
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
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
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

	kubeInformers := kinformers.NewSharedInformerFactory(kubeClient, sdn.ProxyConfig.IPTables.SyncPeriod.Duration)
	internalKubeInformers := kinternalinformers.NewSharedInformerFactory(internalClient, sdn.ProxyConfig.IPTables.SyncPeriod.Duration)
	networkInformers := networkinformers.NewSharedInformerFactory(networkClient, network.DefaultInformerResyncPeriod)

	// you probably want to gate this at some point
	kubeInformers.Core().V1().Services().Informer().AddEventHandler(eventHandlingLogger{})
	kubeInformers.Core().V1().Endpoints().Informer().AddEventHandler(eventHandlingLogger{})

	sdn.informers = &informers{
		KubeClient:     kubeClient,
		NetworkClient:  networkClient,
		InternalClient: internalClient,

		KubeInformers:         kubeInformers,
		InternalKubeInformers: internalKubeInformers,
		NetworkInformers:      networkInformers,
	}
	return nil
}

// start starts the informers.
func (i *informers) start(stopCh <-chan struct{}) {
	i.KubeInformers.Start(stopCh)
	i.InternalKubeInformers.Start(stopCh)
	i.NetworkInformers.Start(stopCh)
}

type eventHandlingLogger struct{}

func describe(obj interface{}) string {
	if svc, ok := obj.(*v1.Service); ok {
		if svc.Spec.Type == v1.ServiceTypeExternalName {
			return fmt.Sprintf("externalname service %s/%s", svc.Namespace, svc.Name)
		} else if svc.Spec.ClusterIP == v1.ClusterIPNone {
			return fmt.Sprintf("headless service %s/%s", svc.Namespace, svc.Name)
		} else {
			return fmt.Sprintf("service %s/%s", svc.Namespace, svc.Name)
		}
	} else if ep, ok := obj.(*v1.Endpoints); ok {
		return fmt.Sprintf("endpoints %s/%s (%d)", ep.Namespace, ep.Name, len(ep.Subsets))
	} else {
		return fmt.Sprintf("%T", obj)
	}
}

func (eventHandlingLogger) OnAdd(obj interface{}) {
	glog.Infof("ADD %s", describe(obj))
}
func (eventHandlingLogger) OnUpdate(oldObj, newObj interface{}) {
	glog.Infof("UPDATE %s -> %s", describe(oldObj), describe(newObj))
}
func (eventHandlingLogger) OnDelete(obj interface{}) {
	glog.Infof("DELETE %s", describe(obj))
}
