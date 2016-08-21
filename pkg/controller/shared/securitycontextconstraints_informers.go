package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	oscache "github.com/openshift/origin/pkg/client/cache"
)

type SecurityContextConstraintsInformer interface {
	Informer() framework.SharedIndexInformer
	Indexer() cache.Indexer
	Lister() *oscache.IndexerToSecurityContextConstraintsLister
}

type securityContextConstraintsInformer struct {
	*sharedInformerFactory
}

func (s *securityContextConstraintsInformer) Informer() framework.SharedIndexInformer {
	s.lock.Lock()
	defer s.lock.Unlock()

	informerObj := &kapi.SecurityContextConstraints{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := s.informers[informerType]
	if exists {
		return informer
	}

	informer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return s.kubeClient.SecurityContextConstraints().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return s.kubeClient.SecurityContextConstraints().Watch(options)
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
