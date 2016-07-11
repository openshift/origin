package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	ocache "github.com/openshift/origin/pkg/client/cache"
)

type PodInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *cache.StoreToPodLister
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

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("pods"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.Pods(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.Pods(kapi.NamespaceAll).Watch(options)
			},
		}

	}

	informer = framework.NewSharedIndexInformer(
		lw,
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

type NodeInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *cache.StoreToNodeLister
}

type nodeInformer struct {
	*sharedInformerFactory
}

func (f *nodeInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.Node{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("nodes"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.Nodes().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.Nodes().Watch(options)
			},
		}

	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *nodeInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *nodeInformer) Lister() *cache.StoreToNodeLister {
	informer := f.Informer()
	return &cache.StoreToNodeLister{Store: informer.GetStore()}
}

type PersistentVolumeInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
}

type persistentVolumeInformer struct {
	*sharedInformerFactory
}

func (f *persistentVolumeInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.PersistentVolume{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("persistentvolumes"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.PersistentVolumes().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.PersistentVolumes().Watch(options)
			},
		}

	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *persistentVolumeInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

type PersistentVolumeClaimInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
}

type persistentVolumeClaimInformer struct {
	*sharedInformerFactory
}

func (f *persistentVolumeClaimInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.PersistentVolumeClaim{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("persistentvolumeclaims"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.PersistentVolumeClaims(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.PersistentVolumeClaims(kapi.NamespaceAll).Watch(options)
			},
		}

	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *persistentVolumeClaimInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

type ReplicationControllerInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *cache.StoreToReplicationControllerLister
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

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("replicationcontrollers"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.ReplicationControllers(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
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

type NamespaceInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *ocache.IndexerToNamespaceLister
}

type namespaceInformer struct {
	*sharedInformerFactory
}

func (f *namespaceInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &kapi.Namespace{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("namespaces"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.kubeClient.Namespaces().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.kubeClient.Namespaces().Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *namespaceInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *namespaceInformer) Lister() *ocache.IndexerToNamespaceLister {
	informer := f.Informer()
	return &ocache.IndexerToNamespaceLister{Indexer: informer.GetIndexer()}
}
