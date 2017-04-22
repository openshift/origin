package shared

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	oscache "github.com/openshift/origin/pkg/client/cache"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImageStreamInformer interface {
	Informer() cache.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.StoreToImageStreamLister
}

type imageStreamInformer struct {
	*sharedInformerFactory
}

func (f *imageStreamInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &imageapi.ImageStream{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return f.originClient.ImageStreams(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return f.originClient.ImageStreams(metav1.NamespaceAll).Watch(options)
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
