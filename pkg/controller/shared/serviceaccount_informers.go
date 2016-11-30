package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	oscache "github.com/openshift/origin/pkg/client/cache"
)

type ServiceAccountInformer interface {
	Informer() cache.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() oscache.StoreToServiceAccountLister
}

type serviceAccountInformer struct {
	*sharedInformerFactory
}

func (s *serviceAccountInformer) Informer() cache.SharedIndexInformer {
	s.lock.Lock()
	defer s.lock.Unlock()

	informerObj := &kapi.ServiceAccount{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := s.informers[informerType]
	if exists {
		return informer
	}

	informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return s.kubeClient.Core().ServiceAccounts(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return s.kubeClient.Core().ServiceAccounts(kapi.NamespaceAll).Watch(options)
			},
		},
		informerObj,
		s.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	s.informers[informerType] = informer

	return informer
}

func (s *serviceAccountInformer) Indexer() cache.Indexer {
	informer := s.Informer()
	return informer.GetIndexer()
}

func (s *serviceAccountInformer) Lister() oscache.StoreToServiceAccountLister {
	informer := s.Informer()
	return oscache.StoreToServiceAccountLister{Indexer: informer.GetIndexer()}
}
