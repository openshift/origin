package controller

import (
	"reflect"
	"sync"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	oclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type InformerFactory interface {
	Start(stopCh <-chan struct{})

	Pods() PodInformer
	ReplicationControllers() ReplicationControllerInformer
	DeploymentConfigs() DeploymentConfigInformer
}

type PodInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *cache.StoreToPodLister
}

type ReplicationControllerInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *cache.StoreToReplicationControllerLister
}

type DeploymentConfigInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.StoreToDeploymentConfigLister
}

func NewInformerFactory(kubeClient kclient.Interface, originClient oclient.Interface, defaultResync time.Duration) InformerFactory {
	return &sharedInformerFactory{
		kubeClient:    kubeClient,
		originClient:  originClient,
		defaultResync: defaultResync,

		informers: map[reflect.Type]framework.SharedIndexInformer{},
	}
}

type sharedInformerFactory struct {
	kubeClient    kclient.Interface
	originClient  oclient.Interface
	defaultResync time.Duration

	informers map[reflect.Type]framework.SharedIndexInformer

	lock sync.Mutex
}

func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	for _, informer := range f.informers {
		go informer.Run(stopCh)
	}
}

func (f *sharedInformerFactory) Pods() PodInformer {
	return &podInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ReplicationControllers() ReplicationControllerInformer {
	return &replicationControllerInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) DeploymentConfigs() DeploymentConfigInformer {
	return &deploymentConfigInformer{sharedInformerFactory: f}
}

type podInformer struct {
	*sharedInformerFactory
}

func (f *podInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.Pod{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.Pods(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.Pods(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *podInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *podInformer) Lister() *cache.StoreToPodLister {
	informer := f.Informer()
	return &cache.StoreToPodLister{Indexer: informer.GetIndexer()}
}

type replicationControllerInformer struct {
	*sharedInformerFactory
}

func (f *replicationControllerInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.ReplicationController{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.ReplicationControllers(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *replicationControllerInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *replicationControllerInformer) Lister() *cache.StoreToReplicationControllerLister {
	informer := f.Informer()
	return &cache.StoreToReplicationControllerLister{Indexer: informer.GetIndexer()}
}

type deploymentConfigInformer struct {
	*sharedInformerFactory
}

func (f *deploymentConfigInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &deployapi.DeploymentConfig{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.DeploymentConfigs(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.DeploymentConfigs(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *deploymentConfigInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *deploymentConfigInformer) Lister() *oscache.StoreToDeploymentConfigLister {
	informer := f.Informer()
	return &oscache.StoreToDeploymentConfigLister{Indexer: informer.GetIndexer()}
}
