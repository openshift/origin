package restoptions

import (
	"sync"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/server/options"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
)

// configRESTOptionsGetter provides RESTOptions based on a provided config
type configRESTOptionsGetter struct {
	masterOptions configapi.MasterConfig

	restOptionsLock sync.Mutex
	restOptionsMap  map[schema.GroupResource]generic.RESTOptions

	storageFactory        serverstorage.StorageFactory
	defaultResourceConfig *serverstorage.ResourceConfig

	cacheEnabled     bool
	defaultCacheSize int
	cacheSizes       map[schema.GroupResource]int
	quorumResources  map[schema.GroupResource]struct{}

	deleteCollectionWorkers int
	enableGarbageCollection bool
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// TODO: this class should either not need to know about configapi.MasterConfig, or not be in pkg/util
func NewConfigGetter(masterOptions configapi.MasterConfig, defaultResourceConfig *serverstorage.ResourceConfig, resourcePrefixOverrides map[schema.GroupResource]string, enforcedStorageVersions map[schema.GroupResource]schema.GroupVersion, quorumResources map[schema.GroupResource]struct{}) (Getter, error) {
	apiserverOptions, err := kubernetes.BuildKubeAPIserverOptions(masterOptions)
	if err != nil {
		return nil, err
	}
	storageFactory, err := kubernetes.BuildStorageFactory(apiserverOptions, enforcedStorageVersions)
	if err != nil {
		return nil, err
	}
	storageFactory.DefaultResourcePrefixes = resourcePrefixOverrides
	storageFactory.StorageConfig.Prefix = masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix

	// perform watch cache heuristic like upstream
	if apiserverOptions.Etcd.EnableWatchCache {
		glog.V(2).Infof("Initializing cache sizes based on %dMB limit", apiserverOptions.GenericServerRunOptions.TargetRAMMB)
		sizes := newHeuristicWatchCacheSizes(apiserverOptions.GenericServerRunOptions.TargetRAMMB)
		if userSpecified, err := options.ParseWatchCacheSizes(apiserverOptions.Etcd.WatchCacheSizes); err == nil {
			for resource, size := range userSpecified {
				sizes[resource] = size
			}
		}
		apiserverOptions.Etcd.WatchCacheSizes, err = options.WriteWatchCacheSizes(sizes)
		if err != nil {
			return nil, err
		}
	}

	cacheSizes, err := options.ParseWatchCacheSizes(apiserverOptions.Etcd.WatchCacheSizes)
	if err != nil {
		return nil, err
	}

	return &configRESTOptionsGetter{
		masterOptions:           masterOptions,
		cacheEnabled:            apiserverOptions.Etcd.EnableWatchCache,
		defaultCacheSize:        apiserverOptions.Etcd.DefaultWatchCacheSize,
		cacheSizes:              cacheSizes,
		restOptionsMap:          map[schema.GroupResource]generic.RESTOptions{},
		defaultResourceConfig:   defaultResourceConfig,
		quorumResources:         quorumResources,
		storageFactory:          storageFactory,
		deleteCollectionWorkers: apiserverOptions.Etcd.DeleteCollectionWorkers,
		enableGarbageCollection: apiserverOptions.Etcd.EnableGarbageCollection,
	}, nil
}

func (g *configRESTOptionsGetter) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	g.restOptionsLock.Lock()
	defer g.restOptionsLock.Unlock()
	if resourceOptions, ok := g.restOptionsMap[resource]; ok {
		return resourceOptions, nil
	}

	config, err := g.storageFactory.NewConfig(resource)
	if err != nil {
		return generic.RESTOptions{}, err
	}

	if _, ok := g.quorumResources[resource]; ok {
		config.Quorum = true
	}

	cacheSize, ok := g.cacheSizes[resource]
	if !ok {
		cacheSize = g.defaultCacheSize
	}

	resourceOptions := generic.RESTOptions{
		StorageConfig:           config,
		Decorator:               registry.StorageWithCacher(cacheSize),
		DeleteCollectionWorkers: g.deleteCollectionWorkers,
		EnableGarbageCollection: g.enableGarbageCollection,
		ResourcePrefix:          g.storageFactory.ResourcePrefix(resource),
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}

// newHeuristicWatchCacheSizes returns a map of suggested watch cache sizes based on total
// memory. It reuses the upstream heuristic and adds OpenShift specific resources.
func newHeuristicWatchCacheSizes(expectedRAMCapacityMB int) map[schema.GroupResource]int {
	// TODO: Revisit this heuristic, copied from upstream
	clusterSize := expectedRAMCapacityMB / 60

	// default enable watch caches for resources that will have a high number of clients accessing it
	// and where the write rate may be significant
	watchCacheSizes := make(map[schema.GroupResource]int)
	watchCacheSizes[schema.GroupResource{Group: "network.openshift.io", Resource: "hostsubnets"}] = maxInt(5*clusterSize, 100)
	watchCacheSizes[schema.GroupResource{Group: "network.openshift.io", Resource: "netnamespaces"}] = maxInt(5*clusterSize, 100)
	watchCacheSizes[schema.GroupResource{Group: "network.openshift.io", Resource: "egressnetworkpolicies"}] = maxInt(10*clusterSize, 100)
	return watchCacheSizes
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
