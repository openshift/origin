package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	oscache "github.com/openshift/origin/pkg/client/cache"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImageStreamInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.StoreToImageStreamLister
}

type imageStreamInformer struct {
	*sharedInformerFactory
}

func (f *imageStreamInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &imageapi.ImageStream{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *imageStreamInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *imageStreamInformer) Lister() *oscache.StoreToImageStreamLister {
	informer := f.Informer()
	return &oscache.StoreToImageStreamLister{Indexer: informer.GetIndexer()}
}
