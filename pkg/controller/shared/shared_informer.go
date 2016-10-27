package shared

import (
	"reflect"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/controller/framework/informers"

	oclient "github.com/openshift/origin/pkg/client"
)

type InformerFactory interface {
	// Start starts informers that can start AFTER the API server and controllers have started
	Start(stopCh <-chan struct{})
	// StartCore starts core informers that must initialize in order for the API server to start
	StartCore(stopCh <-chan struct{})

	Pods() PodInformer
	Namespaces() NamespaceInformer
	Nodes() NodeInformer
	PersistentVolumes() PersistentVolumeInformer
	PersistentVolumeClaims() PersistentVolumeClaimInformer
	ReplicationControllers() ReplicationControllerInformer
	LimitRanges() LimitRangeInformer

	ClusterPolicies() ClusterPolicyInformer
	ClusterPolicyBindings() ClusterPolicyBindingInformer
	Policies() PolicyInformer
	PolicyBindings() PolicyBindingInformer

	DeploymentConfigs() DeploymentConfigInformer
	BuildConfigs() BuildConfigInformer
	ImageStreams() ImageStreamInformer
	SecurityContextConstraints() SecurityContextConstraintsInformer
	ClusterResourceQuotas() ClusterResourceQuotaInformer
	ServiceAccounts() ServiceAccountInformer

	KubernetesInformers() informers.SharedInformerFactory
}

// ListerWatcherOverrides allows a caller to specify special behavior for particular ListerWatchers
// For instance, authentication and authorization types need to go direct to etcd, not through an API server
type ListerWatcherOverrides interface {
	// GetListerWatcher returns back a ListerWatcher for a given resource or nil if
	// no particular ListerWatcher was specified for the type
	GetListerWatcher(resource unversioned.GroupResource) cache.ListerWatcher
}

type DefaultListerWatcherOverrides map[unversioned.GroupResource]cache.ListerWatcher

func (o DefaultListerWatcherOverrides) GetListerWatcher(resource unversioned.GroupResource) cache.ListerWatcher {
	return o[resource]
}

func NewInformerFactory(kubeClient kclient.Interface, originClient oclient.Interface, customListerWatchers ListerWatcherOverrides, defaultResync time.Duration) InformerFactory {
	return &sharedInformerFactory{
		kubeClient:           kubeClient,
		originClient:         originClient,
		customListerWatchers: customListerWatchers,
		defaultResync:        defaultResync,

		informers:            map[reflect.Type]framework.SharedIndexInformer{},
		coreInformers:        map[reflect.Type]framework.SharedIndexInformer{},
		startedInformers:     map[reflect.Type]bool{},
		startedCoreInformers: map[reflect.Type]bool{},
	}
}

type sharedInformerFactory struct {
	kubeClient           kclient.Interface
	originClient         oclient.Interface
	customListerWatchers ListerWatcherOverrides
	defaultResync        time.Duration

	informers            map[reflect.Type]framework.SharedIndexInformer
	coreInformers        map[reflect.Type]framework.SharedIndexInformer
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

func (f *sharedInformerFactory) Pods() PodInformer {
	return &podInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Nodes() NodeInformer {
	return &nodeInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) PersistentVolumes() PersistentVolumeInformer {
	return &persistentVolumeInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) PersistentVolumeClaims() PersistentVolumeClaimInformer {
	return &persistentVolumeClaimInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ReplicationControllers() ReplicationControllerInformer {
	return &replicationControllerInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Namespaces() NamespaceInformer {
	return &namespaceInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) LimitRanges() LimitRangeInformer {
	return &limitRangeInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ClusterPolicies() ClusterPolicyInformer {
	return &clusterPolicyInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ClusterPolicyBindings() ClusterPolicyBindingInformer {
	return &clusterPolicyBindingInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Policies() PolicyInformer {
	return &policyInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) PolicyBindings() PolicyBindingInformer {
	return &policyBindingInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) DeploymentConfigs() DeploymentConfigInformer {
	return &deploymentConfigInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) BuildConfigs() BuildConfigInformer {
	return &buildConfigInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ImageStreams() ImageStreamInformer {
	return &imageStreamInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) SecurityContextConstraints() SecurityContextConstraintsInformer {
	return &securityContextConstraintsInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) ClusterResourceQuotas() ClusterResourceQuotaInformer {
	return &clusterResourceQuotaInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) KubernetesInformers() informers.SharedInformerFactory {
	return kubernetesSharedInformer{f}
}

// TODO: it should use upstream informer as soon #34960 get merged
func (f *sharedInformerFactory) ServiceAccounts() ServiceAccountInformer {
	return &serviceAccountInformer{sharedInformerFactory: f}
}

// kubernetesSharedInformer adapts this informer factory to the identical interface as kubernetes
type kubernetesSharedInformer struct {
	f *sharedInformerFactory
}

func (f kubernetesSharedInformer) Start(ch <-chan struct{})                { f.f.Start(ch) }
func (f kubernetesSharedInformer) Pods() informers.PodInformer             { return f.f.Pods() }
func (f kubernetesSharedInformer) Namespaces() informers.NamespaceInformer { return f.f.Namespaces() }
func (f kubernetesSharedInformer) Nodes() informers.NodeInformer           { return f.f.Nodes() }
func (f kubernetesSharedInformer) PersistentVolumes() informers.PVInformer {
	return f.f.PersistentVolumes()
}
func (f kubernetesSharedInformer) PersistentVolumeClaims() informers.PVCInformer {
	return f.f.PersistentVolumeClaims()
}
