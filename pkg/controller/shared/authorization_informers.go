package shared

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
)

type ClusterPolicyInformer interface {
	Informer() framework.SharedIndexInformer
	// still use an indexer, no telling what someone will want to index on someday
	Indexer() cache.Indexer
	Lister() client.SyncedClusterPoliciesListerInterface
}

type clusterPolicyInformer struct {
	*sharedInformerFactory
}

func (f *clusterPolicyInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &authorizationapi.ClusterPolicy{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.coreInformers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("clusterpolicies"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.ClusterPolicies().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.ClusterPolicies().Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.coreInformers[informerType] = informer

	return informer
}

func (f *clusterPolicyInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *clusterPolicyInformer) Lister() client.SyncedClusterPoliciesListerInterface {
	return &ocache.InformerToClusterPolicyLister{SharedIndexInformer: f.Informer()}
}

type ClusterPolicyBindingInformer interface {
	Informer() framework.SharedIndexInformer
	// still use an indexer, no telling what someone will want to index on someday
	Indexer() cache.Indexer
	Lister() client.SyncedClusterPolicyBindingsListerInterface
}

type clusterPolicyBindingInformer struct {
	*sharedInformerFactory
}

func (f *clusterPolicyBindingInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &authorizationapi.ClusterPolicyBinding{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.coreInformers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("clusterpolicybindings"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.ClusterPolicyBindings().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.ClusterPolicyBindings().Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{},
	)
	f.coreInformers[informerType] = informer

	return informer
}

func (f *clusterPolicyBindingInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *clusterPolicyBindingInformer) Lister() client.SyncedClusterPolicyBindingsListerInterface {
	return &ocache.InformerToClusterPolicyBindingLister{SharedIndexInformer: f.Informer()}
}

type PolicyInformer interface {
	Informer() framework.SharedIndexInformer
	// still use an indexer, no telling what someone will want to index on someday
	Indexer() cache.Indexer
	Lister() client.SyncedPoliciesListerNamespacer
}

type policyInformer struct {
	*sharedInformerFactory
}

func (f *policyInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &authorizationapi.Policy{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.coreInformers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("policies"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.Policies(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.Policies(kapi.NamespaceAll).Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.coreInformers[informerType] = informer

	return informer
}

func (f *policyInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *policyInformer) Lister() client.SyncedPoliciesListerNamespacer {
	return &ocache.InformerToPolicyNamespacer{SharedIndexInformer: f.Informer()}
}

type PolicyBindingInformer interface {
	Informer() framework.SharedIndexInformer
	// still use an indexer, no telling what someone will want to index on someday
	Indexer() cache.Indexer
	Lister() client.SyncedPolicyBindingsListerNamespacer
}

type policyBindingInformer struct {
	*sharedInformerFactory
}

func (f *policyBindingInformer) Informer() framework.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerObj := &authorizationapi.PolicyBinding{}
	informerType := reflect.TypeOf(informerObj)
	informer, exists := f.coreInformers[informerType]
	if exists {
		return informer
	}

	lw := f.customListerWatchers.GetListerWatcher(kapi.Resource("policybindings"))
	if lw == nil {
		lw = &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.originClient.PolicyBindings(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.originClient.PolicyBindings(kapi.NamespaceAll).Watch(options)
			},
		}
	}

	informer = framework.NewSharedIndexInformer(
		lw,
		informerObj,
		f.defaultResync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	f.coreInformers[informerType] = informer

	return informer
}

func (f *policyBindingInformer) Indexer() cache.Indexer {
	informer := f.Informer()
	return informer.GetIndexer()
}

func (f *policyBindingInformer) Lister() client.SyncedPolicyBindingsListerNamespacer {
	return &ocache.InformerToPolicyBindingNamespacer{SharedIndexInformer: f.Informer()}
}
