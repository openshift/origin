package shared

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"

	oscache "github.com/openshift/origin/pkg/client/cache"
)

type SecurityContextConstraintsInformer interface {
	Informer() cache.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.IndexerToSecurityContextConstraintsLister
}

type securityContextConstraintsInformer struct {
	*sharedInformerFactory
}

func (s *securityContextConstraintsInformer) Informer() cache.SharedIndexInformer {
	s.lock.Lock()
	defer s.lock.Unlock()

	informerObj := &kapi.SecurityContextConstraints{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := s.informers[informerType]
	if exists {
		return informer
	}

	informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return s.kubeClient.Core().SecurityContextConstraints().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return s.kubeClient.Core().SecurityContextConstraints().Watch(options)
			},
		},
		informerObj,
		s.defaultResync,
		cache.Indexers{},
	)
	s.informers[informerType] = informer

	return informer
}

func (s *securityContextConstraintsInformer) Indexer() cache.Indexer {
	informer := s.Informer()
	return informer.GetIndexer()
}

func (s *securityContextConstraintsInformer) Lister() *oscache.IndexerToSecurityContextConstraintsLister {
	informer := s.Informer()
	return &oscache.IndexerToSecurityContextConstraintsLister{Indexer: informer.GetIndexer()}
}
