package shared

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"

	oscache "github.com/openshift/origin/pkg/client/cache"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

type TemplateInformer interface {
	Informer() cache.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() oscache.StoreToTemplateLister
}

type templateInformer struct {
	*sharedInformerFactory
}

func (f *templateInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &templateapi.Template{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return f.originClient.Templates(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return f.originClient.Templates(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc, oscache.TemplateUIDIndex: oscache.TemplateUIDIndexFunc},
	)
	f.informers[informerType] = informer

	return informer
}

func (f *templateInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *templateInformer) Lister() oscache.StoreToTemplateLister {
	informer := f.Informer()
	return &oscache.StoreToTemplateListerImpl{Indexer: informer.GetIndexer()}
}
