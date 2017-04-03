package shared

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"

	ocache "github.com/openshift/origin/pkg/client/cache"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type ClusterResourceQuotaInformer interface {
	Informer() cache.SharedIndexInformer
	// still use an indexer, no telling what someone will want to index on someday
	Indexer() cache.Indexer
	Lister() *ocache.IndexerToClusterResourceQuotaLister
}

// clusterResourceQuotaInformer is a core informer because quota needs to be working before the "ensure"
// steps in the API server can complete
type clusterResourceQuotaInformer struct {
	*sharedInformerFactory
}

func (f *clusterResourceQuotaInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &quotaapi.ClusterResourceQuota{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.coreInformers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("clusterresourcequotas"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return f.originClient.ClusterResourceQuotas().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return f.originClient.ClusterResourceQuotas().Watch(options)
			},
		}
	}

	informer = cache.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.coreInformers[informerType] = informer

	return informer
}

func (f *clusterResourceQuotaInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *clusterResourceQuotaInformer) Lister() *ocache.IndexerToClusterResourceQuotaLister {
	return &ocache.IndexerToClusterResourceQuotaLister{Indexer: f.Indexer()}
}
