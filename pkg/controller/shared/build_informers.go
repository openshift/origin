package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	oscache "github.com/openshift/origin/pkg/client/cache"
)

type BuildInformer interface {
	Informer() cache.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.StoreToBuildLister
}

type buildInformer struct {
	*sharedInformerFactory
}

func (f *buildInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &buildapi.Build{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.Builds(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.Builds(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc, oscache.ImageStreamReferenceIndex: oscache.ImageStreamReferenceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *buildInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *buildInformer) Lister() *oscache.StoreToBuildLister {
	informer := f.Informer()
	return &oscache.StoreToBuildLister{Indexer: informer.GetIndexer()}
}
