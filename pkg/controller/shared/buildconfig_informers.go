package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	oscache "github.com/openshift/origin/pkg/client/cache"
)

type BuildConfigInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() oscache.StoreToBuildConfigLister
}

type buildConfigInformer struct {
	*sharedInformerFactory
}

func (f *buildConfigInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &buildapi.BuildConfig{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.BuildConfigs(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.BuildConfigs(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc, oscache.ImageStreamReferenceIndex: oscache.ImageStreamReferenceIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *buildConfigInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *buildConfigInformer) Lister() oscache.StoreToBuildConfigLister {
	informer := f.Informer()
	return &oscache.StoreToBuildConfigListerImpl{Indexer: informer.GetIndexer()}
}
