package shared

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	oclient "github.com/openshift/origin/pkg/client"
)

type InformerFactory interface {
	// Start starts informers that can start AFTER the API server and controllers have started
	Start(stopCh <-chan struct{})
	// StartCore starts core informers that must initialize in order for the API server to start
	StartCore(stopCh <-chan struct{})

	ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)

	BuildConfigs() BuildConfigInformer
	Builds() BuildInformer
	SecurityContextConstraints() SecurityContextConstraintsInformer
	ClusterResourceQuotas() ClusterResourceQuotaInformer

	KubernetesInformers() kinformers.SharedInformerFactory
	InternalKubernetesInformers() kinternalinformers.SharedInformerFactory
}

// ListerWatcherOverrides allows a caller to specify special behavior for particular ListerWatchers
// For instance, authentication and authorization types need to go direct to etcd, not through an API server
type ListerWatcherOverrides interface {
	// GetListerWatcher returns back a ListerWatcher for a given resource or nil if
	// no particular ListerWatcher was specified for the type
	GetListerWatcher(resource schema.GroupResource) cache.ListerWatcher
}

type DefaultListerWatcherOverrides map[schema.GroupResource]cache.ListerWatcher

func (o DefaultListerWatcherOverrides) GetListerWatcher(resource schema.GroupResource) cache.ListerWatcher {
	return o[resource]
}

func NewInformerFactory(
	internalKubeInformers kinternalinformers.SharedInformerFactory,
	kubeInformers kinformers.SharedInformerFactory,
	kubeClient kclientset.Interface,
	originClient oclient.Interface,
	customListerWatchers ListerWatcherOverrides,
	defaultResync time.Duration,
) *sharedInformerFactory {
	return &sharedInformerFactory{
		internalKubeInformers: internalKubeInformers,
		kubeInformers:         kubeInformers,
		kubeClient:            kubeClient,
		originClient:          originClient,
		customListerWatchers:  customListerWatchers,
		defaultResync:         defaultResync,

		informers:            map[reflect.Type]cache.SharedIndexInformer{},
		coreInformers:        map[reflect.Type]cache.SharedIndexInformer{},
		startedInformers:     map[reflect.Type]bool{},
		startedCoreInformers: map[reflect.Type]bool{},
	}
}

type sharedInformerFactory struct {
	internalKubeInformers kinternalinformers.SharedInformerFactory
	kubeInformers         kinformers.SharedInformerFactory
	kubeClient            kclientset.Interface
	originClient          oclient.Interface
	customListerWatchers  ListerWatcherOverrides
	defaultResync         time.Duration

	informers            map[reflect.Type]cache.SharedIndexInformer
	coreInformers        map[reflect.Type]cache.SharedIndexInformer
	startedInformers     map[reflect.Type]bool
	startedCoreInformers map[reflect.Type]bool
	lock                 sync.Mutex
}

func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Run(stopCh)
			f.startedInformers[informerType] = true
		}
	}
}

func (f *sharedInformerFactory) StartCore(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.coreInformers {
		if !f.startedCoreInformers[informerType] {
			go informer.Run(stopCh)
			f.startedCoreInformers[informerType] = true
		}
	}
}

// ForResource unifies the shared informer factory with the generic accessors for GC.
// TODO: as the shared informer factory begins to look like the generated multi-group kube informer, ensure
//   this is refactored to let those informers handle ForResource on their own.
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	if informer, err := f.kubeInformers.ForResource(resource); err == nil {
		return informer, nil
	}

	if resource.Version != runtime.APIVersionInternal {
		// try a generic informer for internal version
		return f.ForResource(schema.GroupVersionResource{Group: resource.Group, Resource: resource.Resource, Version: runtime.APIVersionInternal})
	}

	return nil, fmt.Errorf("no OpenShift shared informer for %s", resource)
}

func (f *sharedInformerFactory) BuildConfigs() BuildConfigInformer {
	return &buildConfigInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Builds() BuildInformer {
	return &buildInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) SecurityContextConstraints() SecurityContextConstraintsInformer {
	return &securityContextConstraintsInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ClusterResourceQuotas() ClusterResourceQuotaInformer {
	return &clusterResourceQuotaInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) KubernetesInformers() kinformers.SharedInformerFactory {
	return f.kubeInformers
}

func (f *sharedInformerFactory) InternalKubernetesInformers() kinternalinformers.SharedInformerFactory {
	return f.internalKubeInformers
}
